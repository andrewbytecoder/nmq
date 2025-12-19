package convert

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "valid duration format",
			input:   "1h30m",
			want:    1*time.Hour + 30*time.Minute,
			wantErr: false,
		},
		{
			name:    "days format",
			input:   "2d3h",
			want:    2*24*time.Hour + 3*time.Hour,
			wantErr: false,
		},
		{
			name:    "only days",
			input:   "5d",
			want:    5 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "numeric value",
			input:   "3600",
			want:    3600 * time.Nanosecond,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "whitespace",
			input:   " 1h ",
			want:    1 * time.Hour,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HumanDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("HumanDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HumanDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
