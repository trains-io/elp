package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/trains-io/elp/apps/z21-gateway/internal/config"
	"github.com/trains-io/elp/apps/z21-gateway/internal/detector"
	"github.com/trains-io/elp/apps/z21-gateway/internal/health"
	"github.com/trains-io/elp/apps/z21-gateway/internal/status"
	"github.com/trains-io/elp/packages/events"
	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

const (
	readTimeout          = 30 * time.Second
	connectRetryInterval = 5 * time.Second
	handshakeTimeout     = 5 * time.Second
	probeTimeout         = 5 * time.Second
)

// Run connects to Z21 and NATS and bridges events until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, healthSrv *health.Server) error {
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()

	var reporter *status.Reporter
	if !cfg.SkipStatus {
		reporter, err = status.NewReporter(cfg.DeviceNamespace, cfg.DeviceName)
		if err != nil {
			return fmt.Errorf("create status reporter: %w", err)
		}
	}

	session := newZ21Session(protocol.DefaultBroadcastFlags)
	trace := newMessageTracer(cfg.LogMessages)
	if cfg.LogMessages {
		slog.Info("z21 message logging enabled")
	}
	if reporter != nil {
		flagsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		flags, err := reporter.BroadcastFlags(flagsCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("read broadcast flags: %w", err)
		}
		session.setFlags(flags)
	}

	controlSub, err := subscribeControl(ctx, nc, cfg.NATSSubjectPrefix, session, trace)
	if err != nil {
		return fmt.Errorf("subscribe control: %w", err)
	}
	defer func() { _ = controlSub.Unsubscribe() }()

	// Readiness reflects the gateway process (NATS, status reporter), not Z21 reachability.
	healthSrv.SetReady(true)
	defer healthSrv.SetReady(false)

	if reporter != nil {
		go status.NewHealthLoop(reporter, cfg.Z21Address).Run(ctx)
	}

	var sessionHW hardwareInfo

	patchSession := func(update status.SessionUpdate) {
		if reporter == nil {
			return
		}
		patchCtx, patchCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer patchCancel()
		if err := reporter.UpdateSession(patchCtx, update); err != nil {
			slog.Warn("failed to patch device status", "error", err)
		}
	}

	markZ21Connected := func(at time.Time) {
		patchSession(status.SessionUpdate{
			LastSeen:            at,
			SerialNumber:        sessionHW.serialNumber,
			FirmwareVersion:     sessionHW.firmwareVersion,
			HardwareType:        sessionHW.hardwareType,
			XBusVersion:         sessionHW.xbusVersion,
			XBusFirmwareVersion: sessionHW.xbusFirmwareVersion,
			LockCode:            sessionHW.lockCode,
		})
	}

	markZ21Disconnected := func(at time.Time) {
		patchSession(status.SessionUpdate{LastSeen: at})
	}

	markZ21Disconnected(time.Now())

	eventsSubject := events.EventsSubject(cfg.NATSSubjectPrefix)
	for {
		if ctx.Err() != nil {
			markZ21Disconnected(time.Now())
			return nil
		}

		z21, err := client.Dial(cfg.Z21Address)
		if err != nil {
			slog.Warn("connect z21 failed, retrying", "error", err)
			markZ21Disconnected(time.Now())
			if !waitForRetry(ctx, connectRetryInterval) {
				markZ21Disconnected(time.Now())
				return nil
			}
			continue
		}

		session.attach(z21)
		hwInfo, err := handshake(ctx, z21, session.flagsValue(), trace)
		if err != nil {
			session.detach()
			_ = z21.Close()
			if ctx.Err() != nil {
				markZ21Disconnected(time.Now())
				return nil
			}
			slog.Warn("z21 handshake failed, retrying", "error", err)
			markZ21Disconnected(time.Now())
			if !waitForRetry(ctx, connectRetryInterval) {
				markZ21Disconnected(time.Now())
				return nil
			}
			continue
		}

		sessionHW = hwInfo
		markZ21Connected(time.Now())
		lost := bridgeZ21(ctx, z21, nc, eventsSubject, cfg, trace, markZ21Connected, markZ21Disconnected, reporter)
		session.detach()
		_ = z21.Close()

		if ctx.Err() != nil {
			markZ21Disconnected(time.Now())
			return nil
		}
		if !lost {
			return nil
		}

		slog.Warn("z21 connection lost, reconnecting")
		markZ21Disconnected(time.Now())
		if !waitForRetry(ctx, connectRetryInterval) {
			markZ21Disconnected(time.Now())
			return nil
		}
	}
}

func bridgeZ21(
	ctx context.Context,
	z21 *client.Client,
	nc *nats.Conn,
	subject string,
	cfg config.Config,
	trace *messageTracer,
	markConnected func(time.Time),
	markDisconnected func(time.Time),
	reporter *status.Reporter,
) (connectionLost bool) {
	var detMgr *detector.Manager
	if reporter != nil {
		mgr, err := detector.NewManager(reporter.Client(), cfg.DeviceNamespace, cfg.DeviceName, cfg.Z21Address, 0)
		if err != nil {
			slog.Warn("can detector manager unavailable", "error", err)
		} else {
			detMgr = mgr
			if err := detMgr.SyncDiscovery(ctx, z21); err != nil {
				slog.Warn("can discovery failed", "error", err)
			}
		}
	}

	for {
		if ctx.Err() != nil {
			return false
		}

		readCtx, readCancel := context.WithTimeout(ctx, readTimeout)
		msgs, err := z21.ReadPacket(readCtx)
		readCancel()
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			if isReadTimeout(err) {
				probeCtx, probeCancel := context.WithTimeout(ctx, probeTimeout)
				_, probeErr := callWithTimeout(probeCtx, z21, protocol.GetHWInfo(), trace)
				probeCancel()
				if probeErr != nil {
					if ctx.Err() != nil {
						return false
					}
					slog.Warn("z21 probe failed after read timeout", "error", probeErr)
					markDisconnected(time.Now())
					return true
				}
				slog.Debug("z21 read timeout but probe ok, device idle")
				markConnected(time.Now())
				if detMgr != nil {
					if err := detMgr.ReconcileAddresses(ctx, z21); err != nil {
						slog.Warn("can address reconcile failed", "error", err)
					}
				}
				continue
			}
			slog.Warn("z21 read error, reconnecting", "error", err)
			markDisconnected(time.Now())
			return true
		}

		trace.rx(msgs...)
		now := time.Now()
		for _, msg := range msgs {
			event := events.Event{
				Timestamp:       now,
				DeviceName:      cfg.DeviceName,
				DeviceNamespace: cfg.DeviceNamespace,
				Header:          msg.Header,
				Data:            msg.Data,
				SubjectPrefix:   cfg.NATSSubjectPrefix,
			}
			payload, err := json.Marshal(event)
			if err != nil {
				slog.Warn("failed to marshal z21 event, skipping packet", "error", err)
				continue
			}
			if err := nc.Publish(subject, payload); err != nil {
				slog.Warn("failed to publish z21 event, retrying on next packet", "error", err)
				continue
			}
		}
		markConnected(now)
	}
}

type hardwareInfo struct {
	hardwareType        string
	serialNumber        string
	firmwareVersion     string
	xbusVersion         string
	xbusFirmwareVersion string
	lockCode            *uint8
}

func handshake(ctx context.Context, z21 *client.Client, broadcastFlags uint32, trace *messageTracer) (hardwareInfo, error) {
	hwMsgs, err := callWithTimeout(ctx, z21, protocol.GetHWInfo(), trace)
	if err != nil {
		return hardwareInfo{}, fmt.Errorf("handshake: %w", err)
	}
	parsed, err := protocol.HWInfoFromMessages(hwMsgs)
	if err != nil {
		return hardwareInfo{}, fmt.Errorf("handshake: %w", err)
	}

	info := hardwareInfo{
		hardwareType:    fmt.Sprintf("0x%08x", parsed.HwType),
		firmwareVersion: protocol.FormatFirmwareVersion(parsed.FirmwareVersion),
	}
	if serialMsgs, err := callWithTimeout(ctx, z21, protocol.GetSerialNumber(), trace); err != nil {
		slog.Warn("z21 serial number unavailable", "error", err)
	} else if serial, ok := protocol.SerialFromMessages(serialMsgs); ok {
		info.serialNumber = serial
	}
	if xMsgs, err := callWithTimeout(ctx, z21, protocol.GetXVersion(), trace); err != nil {
		slog.Warn("z21 x-bus version unavailable", "error", err)
	} else if xver, err := protocol.XVersionFromMessages(xMsgs); err != nil {
		slog.Warn("z21 x-bus version parse failed", "error", err)
	} else {
		info.xbusVersion = protocol.FormatXBusVersion(xver.XBusVersion)
	}
	if fwMsgs, err := callWithTimeout(ctx, z21, protocol.GetXFirmware(), trace); err != nil {
		slog.Warn("z21 x-bus firmware unavailable", "error", err)
	} else if fw, err := protocol.XFirmwareFromMessages(fwMsgs); err != nil {
		slog.Warn("z21 x-bus firmware parse failed", "error", err)
	} else {
		info.xbusFirmwareVersion = protocol.FormatXFirmwareVersion(fw)
	}
	if codeMsgs, err := callWithTimeout(ctx, z21, protocol.GetCode(), trace); err != nil {
		slog.Warn("z21 lock code unavailable", "error", err)
	} else if code, err := protocol.CodeFromMessages(codeMsgs); err != nil {
		slog.Warn("z21 lock code parse failed", "error", err)
	} else {
		info.lockCode = &code
	}

	flags := broadcastFlags
	if flags == 0 {
		flags = protocol.DefaultBroadcastFlags
	}
	if err := sendWithTimeout(ctx, z21, protocol.SetBroadcastFlags(flags), trace); err != nil {
		return hardwareInfo{}, fmt.Errorf("set broadcast flags: %w", err)
	}
	if _, err := callWithTimeout(ctx, z21, protocol.SystemStateGetData(), trace); err != nil {
		return hardwareInfo{}, fmt.Errorf("system state: %w", err)
	}
	return info, nil
}

func callWithTimeout(ctx context.Context, z21 *client.Client, req protocol.Message, trace *messageTracer) ([]protocol.Message, error) {
	trace.tx(req)
	stepCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	msgs, err := z21.Call(stepCtx, req)
	if err != nil {
		return nil, err
	}
	trace.rx(msgs...)
	return msgs, nil
}

func sendWithTimeout(ctx context.Context, z21 *client.Client, req protocol.Message, trace *messageTracer) error {
	trace.tx(req)
	stepCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	return z21.Send(stepCtx, req)
}

func waitForRetry(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func isReadTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
