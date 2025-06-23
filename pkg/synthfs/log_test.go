package synthfs_test

import (
	"bytes"
	"os"
	"regexp"
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
	// Check for synthfs identifier
	// Note: zerolog's default console writer might not use "lib":"synthfs" format strictly like JSON.
	// It usually includes fields in a human-readable way.
	// For this test, we'll check for the presence of "lib=synthfs" (possibly with ANSI codes) for console output
	// or "\"lib\":\"synthfs\"" for JSON output.
	// The regex allows for ANSI escape codes around "synthfs".
	libSynthfsRegex := regexp.MustCompile(`lib=(\x1b\[[0-9;]*m)*synthfs(\x1b\[0m)*`)
	if !libSynthfsRegex.MatchString(output) && !strings.Contains(output, "\"lib\":\"synthfs\"") {
		t.Errorf("Expected log output to contain 'lib=synthfs' (possibly with ANSI codes) or '\"lib\":\"synthfs\"', got: %s", output)
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

func TestTraceLevelLogging(t *testing.T) {
	var buf bytes.Buffer
	synthfs.SetLogOutput(&buf)
	synthfs.SetLogLevel(zerolog.TraceLevel) // Ensure trace level is enabled

	// Example complex data structure
	type Detail struct {
		Key   string
		Value int
	}
	data := struct {
		Name    string
		Details []Detail
		Nested  map[string]string
	}{
		Name:    "TraceTest",
		Details: []Detail{{"A", 1}, {"B", 2}},
		Nested:  map[string]string{"k1": "v1", "k2": "v2"},
	}

	// Log the data structure at trace level
	// Using a generic way to log, assuming logger might be used directly
	// or a specific trace function might exist. For now, using general logger.
	synthfs.Logger().Trace().Interface("complex_data", data).Msg("logging complex data for trace test")

	output := buf.String()

	// Verify that the log output contains parts of the complex data structure.
	// This is a basic check; more specific checks might be needed depending on
	// how "full data dumps" are implemented (e.g., JSON marshaling).
	if !strings.Contains(output, "TraceTest") {
		t.Errorf("Expected trace output to contain 'TraceTest', got: %s", output)
	}
	if !strings.Contains(output, "\"Key\":\"A\"") && !strings.Contains(output, "Key:A") { // Check for JSON-like or struct-like output
		t.Errorf("Expected trace output to contain details like '\"Key\":\"A\"' or 'Key:A', got: %s", output)
	}
	if !strings.Contains(output, "\"k1\":\"v1\"") && !strings.Contains(output, "k1:v1") { // Check for map content
		t.Errorf("Expected trace output to contain nested map content like '\"k1\":\"v1\"' or 'k1:v1', got: %s", output)
	}
	if !strings.Contains(output, "logging complex data for trace test") {
		t.Errorf("Expected trace output to contain the message 'logging complex data for trace test', got: %s", output)
	}

	// Reset log level for subsequent tests
	synthfs.SetLogLevel(zerolog.WarnLevel)
}

func TestLogOutputRedirection(t *testing.T) {
	// The library initializes with os.Stderr. We restore to os.Stderr after the test.
	// This ensures that this test cleans up after itself.
	// If other tests modify global log output without cleanup, it could affect subsequent tests,
	// but this specific test will ensure it reverts its own changes to os.Stderr.
	originalDefaultWriter := os.Stderr // Default writer used by init()
	defer synthfs.SetLogOutput(originalDefaultWriter)

	synthfs.SetLogLevel(zerolog.InfoLevel)

	// Define regex once for use in both file and stderr checks
	// Regex for lib=synthfs with optional ANSI codes (copied from TestLoggingSetup)
	libSynthfsRegex := regexp.MustCompile(`lib=(\x1b\[[0-9;]*m)*synthfs(\x1b\[0m)*`)

	// Test redirection to a file
	logFile, err := os.CreateTemp(t.TempDir(), "test_synthfs_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp log file: %v", err)
	}
	logFileName := logFile.Name()
	synthfs.SetLogOutput(logFile)

	logMsgFile := "message logged to file"
	synthfs.Logger().Info().Msg(logMsgFile)
	logFile.Close() // Close the file to ensure content is flushed

	fileContentBytes, err := os.ReadFile(logFileName)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	fileContentStr := string(fileContentBytes)

	lines := strings.Split(fileContentStr, "\n")
	foundLogMsgFile := false
	foundLibIdentifierInLine := false
	// libSynthfsRegex is already defined at the top of the function

	for _, line := range lines {
		if strings.Contains(line, logMsgFile) {
			foundLogMsgFile = true
			// Check this specific line for the lib identifier using regex for file
			if libSynthfsRegex.MatchString(line) || strings.Contains(line, "\"lib\":\"synthfs\"") {
				foundLibIdentifierInLine = true
			}
			// For debugging the exact line:
			// t.Logf("Found line for '%s': %s", logMsgFile, line)
			break
		}
	}

	if !foundLogMsgFile {
		t.Errorf("Expected log file to contain the message '%s'. Full content:\n%s", logMsgFile, fileContentStr)
	}
	if !foundLibIdentifierInLine && foundLogMsgFile { // Only error if message was found but lib id was missing in that line
		// Find the specific line again to include in the error message
		var targetLine string
		for _, line := range lines {
			if strings.Contains(line, logMsgFile) {
				targetLine = line
				break
			}
		}
		t.Errorf("Log line for '%s' found, but it did not contain 'lib=synthfs' (with or without ANSI codes) or '\"lib\":\"synthfs\"'. Line content: '%s'", logMsgFile, targetLine)
	}


	// Test resetting to default (os.Stderr)
	// We'll capture os.Stderr to verify this. This is a bit tricky.
	// A common way is to temporarily replace os.Stderr with a pipe.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr // Restore stderr
	}()

	synthfs.SetLogOutput(os.Stderr) // Reset to default or explicitly set to os.Stderr
	// If there's a specific function like synthfs.ResetLogOutput(), that would be better.
	// Assuming SetLogOutput(os.Stderr) is the way to go back to default.

	logMsgStderr := "message logged to stderr"
	synthfs.Logger().Info().Msg(logMsgStderr)
	w.Close() // Close the writer to unblock the reader

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	r.Close()

	stderrOutput := stderrBuf.String()
	if !strings.Contains(stderrOutput, logMsgStderr) {
		t.Errorf("Expected stderr output to contain '%s', got: %s", logMsgStderr, stderrOutput)
	}
	// libSynthfsRegex is defined at the top of the function and should be used here.
	// Ensure no re-declaration like: libSynthfsRegex := ...
	if !libSynthfsRegex.MatchString(stderrOutput) && !strings.Contains(stderrOutput, "\"lib\":\"synthfs\"") {
		t.Errorf("Expected stderr output to contain lib identifier, got: %s", stderrOutput)
	}

	// Clean up: SetLogOutput will be called by defer, restoring originalOutput.
	// The temp file is cleaned up by t.TempDir().
}
