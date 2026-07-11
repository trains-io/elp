package k8s

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/trains-io/elp/apps/api/internal/api"
	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// StartDeviceWatch watches Z21Device resources and publishes changes to hub.
func StartDeviceWatch(ctx context.Context, hub *api.StreamHub) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("kubernetes config: %w", err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(z21v1alpha1.AddToScheme(scheme))

	c, err := cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("create cache: %w", err)
	}

	informer, err := c.GetInformer(ctx, &z21v1alpha1.Z21Device{})
	if err != nil {
		return fmt.Errorf("get informer: %w", err)
	}

	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			publishDevice(hub, obj, "updated")
		},
		UpdateFunc: func(_, newObj any) {
			publishDevice(hub, newObj, "updated")
		},
		DeleteFunc: func(obj any) {
			device, ok := obj.(*z21v1alpha1.Z21Device)
			if !ok {
				return
			}
			hub.Publish(device.Namespace, api.DeviceStreamEvent{
				Type: "deleted",
				Name: device.Name,
			})
		},
	})
	if err != nil {
		return fmt.Errorf("add event handler: %w", err)
	}

	go func() {
		if err := c.Start(ctx); err != nil {
			slog.Error("device watch cache stopped", "error", err)
		}
	}()

	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return ctx.Err()
	}

	slog.Info("device watch started")
	return nil
}

func publishDevice(hub *api.StreamHub, obj any, eventType string) {
	device, ok := obj.(*z21v1alpha1.Z21Device)
	if !ok {
		return
	}
	hub.Publish(device.Namespace, api.DeviceStreamEvent{
		Type:   eventType,
		Device: api.DeviceFromCR(device),
	})
}
