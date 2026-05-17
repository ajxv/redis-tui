package tui_test

import (
	"testing"

	"github.com/ajxv/redis-tui/internal/tui"
)

func TestParseRedisURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    tui.ParsedURL
		wantErr bool
	}{
		{
			name:  "plain host no port",
			input: "redis://localhost",
			want:  tui.ParsedURL{Host: "localhost:6379", DB: 0, TLS: false},
		},
		{
			name:  "host with port",
			input: "redis://localhost:6380",
			want:  tui.ParsedURL{Host: "localhost:6380", DB: 0, TLS: false},
		},
		{
			name:  "password only",
			input: "redis://:secret@localhost",
			want:  tui.ParsedURL{Host: "localhost:6379", Password: "secret", DB: 0, TLS: false},
		},
		{
			name:  "username and password",
			input: "redis://alice:secret@localhost:6380/2",
			want:  tui.ParsedURL{Host: "localhost:6380", Username: "alice", Password: "secret", DB: 2, TLS: false},
		},
		{
			name:  "rediss implies TLS",
			input: "rediss://localhost",
			want:  tui.ParsedURL{Host: "localhost:6379", DB: 0, TLS: true},
		},
		{
			name:  "rediss with port and db",
			input: "rediss://host.example.com:6380/3",
			want:  tui.ParsedURL{Host: "host.example.com:6380", DB: 3, TLS: true},
		},
		{
			name:  "db index zero explicit",
			input: "redis://localhost/0",
			want:  tui.ParsedURL{Host: "localhost:6379", DB: 0, TLS: false},
		},
		{
			name:    "invalid scheme",
			input:   "ftp://localhost",
			wantErr: true,
		},
		{
			name:    "non-numeric db",
			input:   "redis://localhost/notanumber",
			wantErr: true,
		},
		{
			name:    "negative db",
			input:   "redis://localhost/-1",
			wantErr: true,
		},
		{
			name:    "missing host",
			input:   "redis://",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tui.ParseRedisURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil (result: %+v)", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got.Host != tc.want.Host {
				t.Errorf("Host: got %q, want %q", got.Host, tc.want.Host)
			}
			if got.Password != tc.want.Password {
				t.Errorf("Password: got %q, want %q", got.Password, tc.want.Password)
			}
			if got.Username != tc.want.Username {
				t.Errorf("Username: got %q, want %q", got.Username, tc.want.Username)
			}
			if got.DB != tc.want.DB {
				t.Errorf("DB: got %d, want %d", got.DB, tc.want.DB)
			}
			if got.TLS != tc.want.TLS {
				t.Errorf("TLS: got %v, want %v", got.TLS, tc.want.TLS)
			}
		})
	}
}
