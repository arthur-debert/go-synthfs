package synthfs_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := synthfs.NewLogger(&buf, zerolog.InfoLevel)

	logger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}

	t.Logf("Log output: %q", output)
	if !strings.HasSuffix(strings.TrimSpace(output), "lib=synthfs") {
		t.Errorf("Expected log output to end with 'lib=synthfs', got: %s", output)
	}
}

func TestLogLevelFromString(t *testing.T) {
	testCases := []struct {
		levelStr string
		expected zerolog.Level
		wantErr  bool
	}{
		{"trace", zerolog.TraceLevel, false},
		{"debug", zerolog.DebugLevel, false},
		{"info", zerolog.InfoLevel, false},
		{"warn", zerolog.WarnLevel, false},
		{"error", zerolog.ErrorLevel, false},
		{"invalid", zerolog.NoLevel, true},
	}

	for _, tc := range testCases {
		t.Run(tc.levelStr, func(t *testing.T) {
			level, err := synthfs.LogLevelFromString(tc.levelStr)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for invalid level %q", tc.levelStr)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if level != tc.expected {
				t.Errorf("Expected level %v, got %v", tc.expected, level)
			}
		})
	}
}

func TestNewTestLogger(t *testing.T) {
	testCases := []struct {
		verbose  int
		expected zerolog.Level
	}{
		{0, zerolog.WarnLevel},
		{1, zerolog.InfoLevel},
		{2, zerolog.DebugLevel},
		{3, zerolog.TraceLevel},
		{4, zerolog.TraceLevel}, // Should cap at trace
	}

	for _, tc := range testCases {
		t.Run("verbose_"+string(rune(tc.verbose+'0')), func(t *testing.T) {
			var buf bytes.Buffer
			logger := synthfs.NewTestLogger(&buf, tc.verbose)
			if logger.GetLevel() != tc.expected {
				t.Errorf("Expected level %v for verbose %d, got %v", tc.expected, tc.verbose, logger.GetLevel())
			}
		})
	}
}