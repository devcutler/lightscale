package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	socketPath string
}

func New(socketPath string) *Client {
	if envURL := os.Getenv("LIGHTSCALE_URL"); envURL != "" {
		return &Client{
			httpClient: &http.Client{Timeout: 30 * time.Second},
			baseURL:    strings.TrimRight(envURL, "/"),
		}
	}
	if socketPath == "" {
		socketPath = os.Getenv("LIGHTSCALE_SOCKET")
	}
	if socketPath == "" {
		socketPath = defaultSocketPath()
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
		baseURL:    "http://lightscale",
		socketPath: socketPath,
	}
}
func (c *Client) SocketPath() string { return c.socketPath }

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api: status %d: %s", e.Status, e.Body)
}
func (c *Client) IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
func (c *Client) GetText(ctx context.Context, path string) (string, error) {
	resp, err := c.send(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("client: read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return string(body), nil
}
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.send(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("client: decode: %w", err)
	}
	return nil
}
func (c *Client) send(ctx context.Context, method, path string, body any) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("client: bad path %q: %w", path, err)
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("client: encode body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %s %s: %w", method, path, err)
	}
	return resp, nil
}

func defaultSocketPath() string {

	return "/run/lightscale/lightscale.sock"
}
