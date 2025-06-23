package synthfs_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestLoggingSetup(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	synthfs.SetLogOutput(&buf)

	// Test setting log level
	synthfs.SetLogLevel(zerolog.InfoLevel)
	if synthfs.GetLogLevel() != zerolog.InfoLevel {
		t.Errorf("Expected log level Info, got %v", synthfs.GetLogLevel())
	}

	// Test logging a message
	logger := synthfs.Logger()
	logger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}
	// Check for synthfs identifier (may be formatted differently by console writer)
	if !strings.Contains(output, "synthfs") {
		t.Errorf("Expected log output to contain 'synthfs', got: %s", output)
	}
}

func TestSetLogLevelFromString(t *testing.T) {
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
			err := synthfs.SetLogLevelFromString(tc.levelStr)

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

			if synthfs.GetLogLevel() != tc.expected {
				t.Errorf("Expected level %v, got %v", tc.expected, synthfs.GetLogLevel())
			}
		})
	}
}

func TestSetupTestLogging(t *testing.T) {
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
			synthfs.SetupTestLogging(tc.verbose)

			if synthfs.GetLogLevel() != tc.expected {
				t.Errorf("Expected level %v for verbose %d, got %v", tc.expected, tc.verbose, synthfs.GetLogLevel())
			}
		})
	}
}

func TestOperationLogging(t *testing.T) {
	var buf bytes.Buffer
	synthfs.SetLogOutput(&buf)
	synthfs.SetLogLevel(zerolog.InfoLevel)

	opID := synthfs.OperationID("test-op-123")

	// Test operation start logging
	synthfs.LogOperationStart(opID, "CreateFile", "/test/path")

	output := buf.String()
	if !strings.Contains(output, "operation started") {
		t.Errorf("Expected 'operation started' in log output")
	}
	if !strings.Contains(output, "test-op-123") {
		t.Errorf("Expected operation ID in log output")
	}
	if !strings.Contains(output, "CreateFile") {
		t.Errorf("Expected operation type in log output")
	}

	// Clear buffer for next test
	buf.Reset()

	// Test operation completion logging
	synthfs.LogOperationComplete(opID, "CreateFile", "/test/path", true, 100*time.Millisecond)

	output = buf.String()
	if !strings.Contains(output, "operation completed successfully") {
		t.Errorf("Expected 'operation completed successfully' in log output")
	}
	// Check for success indicator (console writer may format booleans differently)
	if !strings.Contains(output, "true") {
		t.Errorf("Expected success indicator 'true' in log output, got: %s", output)
	}
}

func TestValidationLogging(t *testing.T) {
	var buf bytes.Buffer
	synthfs.SetLogOutput(&buf)
	synthfs.SetLogLevel(zerolog.DebugLevel)

	opID := synthfs.OperationID("test-validation")

	// Test validation result logging
	synthfs.LogValidationResult(opID, "CreateFile", "/test/path", false, "file already exists")

	output := buf.String()
	if !strings.Contains(output, "operation validation result") {
		t.Errorf("Expected 'operation validation result' in log output")
	}
	// Check for validation failure indicator
	if !strings.Contains(output, "false") {
		t.Errorf("Expected validation failure indicator 'false' in log output, got: %s", output)
	}
	if !strings.Contains(output, "file already exists") {
		t.Errorf("Expected validation reason in log output")
	}
}

func TestDisableLogging(t *testing.T) {
	var buf bytes.Buffer
	synthfs.SetLogOutput(&buf)

	// Disable logging
	synthfs.DisableLogging()

	// Try to log something
	logger := synthfs.Logger()
	logger.Info().Msg("this should not appear")

	output := buf.String()
	if strings.Contains(output, "this should not appear") {
		t.Errorf("Expected no log output when logging is disabled, got: %s", output)
	}

	// Re-enable for other tests
	synthfs.SetLogLevel(zerolog.WarnLevel)
}
