package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client talks to the elp HTTP API.
type Client struct {
	BaseURL   string
	Namespace string
	HTTP      *http.Client
}

func New(baseURL, namespace string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Namespace: namespace,
		HTTP:      http.DefaultClient,
	}
}

func (c *Client) CreateDevice(ctx context.Context, req DeviceCreate) (Device, error) {
	var out Device
	err := c.doJSON(ctx, http.MethodPost, c.devicesPath(), req, http.StatusCreated, &out)
	return out, err
}

func (c *Client) GetDevice(ctx context.Context, name string) (Device, error) {
	var out Device
	err := c.doJSON(ctx, http.MethodGet, c.devicePath(name), nil, http.StatusOK, &out)
	return out, err
}

func (c *Client) ListDevices(ctx context.Context) (DeviceList, error) {
	var out DeviceList
	err := c.doJSON(ctx, http.MethodGet, c.devicesPath(), nil, http.StatusOK, &out)
	return out, err
}

// WatchDevices streams SSE device events until ctx is cancelled.
func (c *Client) WatchDevices(ctx context.Context, fn func(StreamEvent) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.streamPath(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeAPIError(resp)
	}

	return readSSE(resp.Body, fn)
}

func (c *Client) devicesPath() string {
	return fmt.Sprintf("%s/api/v1/namespaces/%s/devices", c.BaseURL, url.PathEscape(c.Namespace))
}

func (c *Client) devicePath(name string) string {
	return fmt.Sprintf("%s/%s", c.devicesPath(), url.PathEscape(name))
}

func (c *Client) streamPath() string {
	return fmt.Sprintf("%s/stream", c.devicesPath())
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, expectStatus int, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectStatus {
		return decodeAPIError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil || apiErr.Error == "" {
		return fmt.Errorf("api request failed: %s", resp.Status)
	}
	return fmt.Errorf("%s", apiErr.Error)
}

func readSSE(r io.Reader, fn func(StreamEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var eventType string
	var dataLines []string

	flush := func() error {
		if eventType == "" && len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		eventType = ""
		dataLines = nil
		if payload == "" {
			return nil
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return fmt.Errorf("decode stream event: %w", err)
		}
		if event.Type == "" {
			event.Type = eventType
		}
		return fn(event)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if after, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = strings.TrimSpace(after)
			continue
		}
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			dataLines = append(dataLines, after)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
