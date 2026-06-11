package client

import "testing"

func TestNewTargetResolution(t *testing.T) {
	const defaultSock = "/run/lightscale/lightscale.sock"

	tests := []struct {
		name       string
		envURL     string
		envSocket  string
		socketArg  string
		wantBase   string
		wantSocket string
	}{
		{
			name:       "default when nothing set",
			wantBase:   "http://lightscale",
			wantSocket: defaultSock,
		},
		{
			name:       "explicit socket arg",
			socketArg:  "/tmp/custom.sock",
			wantBase:   "http://lightscale",
			wantSocket: "/tmp/custom.sock",
		},
		{
			name:       "LIGHTSCALE_SOCKET env",
			envSocket:  "/tmp/env.sock",
			wantBase:   "http://lightscale",
			wantSocket: "/tmp/env.sock",
		},
		{
			name:       "explicit socket arg beats LIGHTSCALE_SOCKET env",
			socketArg:  "/tmp/flag.sock",
			envSocket:  "/tmp/env.sock",
			wantBase:   "http://lightscale",
			wantSocket: "/tmp/flag.sock",
		},
		{
			name:       "LIGHTSCALE_URL wins over everything and leaves socket empty",
			envURL:     "http://127.0.0.1:9999/",
			envSocket:  "/tmp/env.sock",
			socketArg:  "/tmp/flag.sock",
			wantBase:   "http://127.0.0.1:9999",
			wantSocket: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			t.Setenv("LIGHTSCALE_URL", tc.envURL)
			t.Setenv("LIGHTSCALE_SOCKET", tc.envSocket)

			c := New(tc.socketArg)
			if c.baseURL != tc.wantBase {
				t.Errorf("baseURL = %q, want %q", c.baseURL, tc.wantBase)
			}
			if got := c.SocketPath(); got != tc.wantSocket {
				t.Errorf("SocketPath() = %q, want %q", got, tc.wantSocket)
			}
		})
	}
}
