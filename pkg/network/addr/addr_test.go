// E:\work\ncp\pkg\api\addr\addr_test.go
package addr

import (
	"testing"
)

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "valid host and port",
			input:    "localhost:8080",
			wantHost: "localhost",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name:     "valid ip and port",
			input:    "127.0.0.1:3000",
			wantHost: "127.0.0.1",
			wantPort: 3000,
			wantErr:  false,
		},
		{
			name:     "invalid format - missing port",
			input:    "localhost",
			wantHost: "",
			wantPort: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - multiple colons",
			input:    "localhost:8080:extra",
			wantHost: "",
			wantPort: 0,
			wantErr:  true,
		},
		{
			name:     "invalid port - non numeric",
			input:    "localhost:abc",
			wantHost: "",
			wantPort: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			wantHost: "",
			wantPort: 0,
			wantErr:  true,
		},
		{
			name:     "common port",
			input:    ":8080",
			wantHost: "",
			wantPort: 8080,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPort, err := SplitHostPort(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("SplitHostPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotHost != tt.wantHost {
				t.Errorf("SplitHostPort() gotHost = %v, want %v", gotHost, tt.wantHost)
			}

			if gotPort != tt.wantPort {
				t.Errorf("SplitHostPort() gotPort = %v, want %v", gotPort, tt.wantPort)
			}
		})
	}
}
