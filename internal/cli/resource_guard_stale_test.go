package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseEtime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"01:30", 1*time.Minute + 30*time.Second},
		{"05:00:30", 5*time.Hour + 30*time.Second},
		{"2-03:04:05", 2*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second},
		{"00:05", 5 * time.Second},
		{"1-00:00:00", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseEtime(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
