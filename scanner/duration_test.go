package scanner

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 2 * 7 * 24 * time.Hour, false},
		{"1m", 30 * 24 * time.Hour, false}, // regression: must be months, not 1 minute
		{"12m", 360 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false}, // combo form, not matched by custom regex
		{"90s", 90 * time.Second, false},
		{"", 0, true},
		{"bogus", 0, true},
		{"1x", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseDuration(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDuration(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
