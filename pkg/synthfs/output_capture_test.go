package synthfs_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestShellCommand_OutputCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("capture stdout", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a command that produces stdout
		op := sfs.ShellCommand("echo 'Hello, World!'", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		if len(ops) != 1 {
			t.Fatalf("Expected 1 operation, got %d", len(ops))
		}

		opResult := ops[0].(synthfs.OperationResult)
		
		// Check stdout was captured
		stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
		expectedOutput := "Hello, World!\n"
		if stdout != expectedOutput {
			t.Errorf("Expected stdout %q, got %q", expectedOutput, stdout)
		}

		// stderr should be empty
		stderr := synthfs.GetOperationOutput(opResult.Operation, "stderr")
		if stderr != "" {
			t.Errorf("Expected empty stderr, got %q", stderr)
		}
	})

	t.Run("capture stderr", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a command that produces stderr
		op := sfs.ShellCommand("echo 'Error message' >&2", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Check stderr was captured
		stderr := synthfs.GetOperationOutput(opResult.Operation, "stderr")
		expectedError := "Error message\n"
		if stderr != expectedError {
			t.Errorf("Expected stderr %q, got %q", expectedError, stderr)
		}

		// stdout should be empty
		stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
		if stdout != "" {
			t.Errorf("Expected empty stdout, got %q", stdout)
		}
	})

	t.Run("capture both stdout and stderr", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a command that produces both stdout and stderr
		op := sfs.ShellCommand("echo 'Output'; echo 'Error' >&2", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Check both were captured
		stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
		stderr := synthfs.GetOperationOutput(opResult.Operation, "stderr")
		
		if stdout != "Output\n" {
			t.Errorf("Expected stdout 'Output\\n', got %q", stdout)
		}
		if stderr != "Error\n" {
			t.Errorf("Expected stderr 'Error\\n', got %q", stderr)
		}
	})

	t.Run("no capture when option not set", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a command without capture option
		op := sfs.ShellCommand("echo 'Not captured'", 
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Check nothing was captured
		stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
		stderr := synthfs.GetOperationOutput(opResult.Operation, "stderr")
		
		if stdout != "" {
			t.Errorf("Expected no stdout capture, got %q", stdout)
		}
		if stderr != "" {
			t.Errorf("Expected no stderr capture, got %q", stderr)
		}
	})

	t.Run("capture multiline output", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a command with multiline output
		op := sfs.ShellCommand("printf 'Line 1\\nLine 2\\nLine 3\\n'", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Check multiline output was captured
		stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
		expectedOutput := "Line 1\nLine 2\nLine 3\n"
		if stdout != expectedOutput {
			t.Errorf("Expected stdout %q, got %q", expectedOutput, stdout)
		}
	})
}

func TestCustomOperation_OutputCapture(t *testing.T) {
	t.Run("custom operation with output", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create custom operation that stores output
		op := sfs.CustomOperationWithOutput("data-processor",
			func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error {
				// Simulate some processing
				data := []string{"apple", "banana", "cherry"}
				
				// Store various outputs
				storeOutput("items", strings.Join(data, ","))
				storeOutput("count", len(data))
				storeOutput("processed", true)
				
				return nil
			})

		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		if len(ops) != 1 {
			t.Fatalf("Expected 1 operation, got %d", len(ops))
		}

		opResult := ops[0].(synthfs.OperationResult)
		
		// Check stored outputs
		items := synthfs.GetOperationOutput(opResult.Operation, "items")
		if items != "apple,banana,cherry" {
			t.Errorf("Expected items 'apple,banana,cherry', got %q", items)
		}

		// Check non-string output
		count := synthfs.GetOperationOutputValue(opResult.Operation, "count")
		if count != 3 {
			t.Errorf("Expected count 3, got %v", count)
		}

		processed := synthfs.GetOperationOutputValue(opResult.Operation, "processed")
		if processed != true {
			t.Errorf("Expected processed true, got %v", processed)
		}
	})

	t.Run("custom operation with complex output", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create custom operation that stores complex data
		op := sfs.CustomOperationWithOutput("analyzer",
			func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error {
				// Store different types of data
				storeOutput("result", map[string]interface{}{
					"status": "success",
					"score":  95.5,
					"tags":   []string{"important", "verified"},
				})
				
				storeOutput("metadata", struct {
					Version string
					Author  string
				}{
					Version: "1.0.0",
					Author:  "test",
				})
				
				return nil
			})

		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Check complex outputs
		result1 := synthfs.GetOperationOutputValue(opResult.Operation, "result")
		if result1 == nil {
			t.Error("Expected result to be stored")
		}
		
		if resultMap, ok := result1.(map[string]interface{}); ok {
			if resultMap["status"] != "success" {
				t.Errorf("Expected status 'success', got %v", resultMap["status"])
			}
			if resultMap["score"] != 95.5 {
				t.Errorf("Expected score 95.5, got %v", resultMap["score"])
			}
		} else {
			t.Error("Result is not a map")
		}

		metadata := synthfs.GetOperationOutputValue(opResult.Operation, "metadata")
		if metadata == nil {
			t.Error("Expected metadata to be stored")
		}
	})

	t.Run("get all outputs", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create custom operation with multiple outputs
		op := sfs.CustomOperationWithOutput("multi-output",
			func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error {
				storeOutput("key1", "value1")
				storeOutput("key2", 42)
				storeOutput("key3", true)
				return nil
			})

		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}

		// Get the operation result
		ops := result.GetOperations()
		opResult := ops[0].(synthfs.OperationResult)
		
		// Get all outputs
		allOutputs := synthfs.GetAllOperationOutputs(opResult.Operation)
		
		// Should have at least our 3 outputs (might have more like description)
		if len(allOutputs) < 3 {
			t.Errorf("Expected at least 3 outputs, got %d", len(allOutputs))
		}

		// Check specific outputs
		if allOutputs["key1"] != "value1" {
			t.Errorf("Expected key1='value1', got %v", allOutputs["key1"])
		}
		if allOutputs["key2"] != 42 {
			t.Errorf("Expected key2=42, got %v", allOutputs["key2"])
		}
		if allOutputs["key3"] != true {
			t.Errorf("Expected key3=true, got %v", allOutputs["key3"])
		}
	})

	t.Run("custom operation with validation and output", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)

		// Create the base custom operation with output
		op := synthfs.NewCustomOperationWithOutput("validated-op",
			func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error {
				storeOutput("status", "executed")
				return nil
			})
		
		// Add validation
		op = op.WithValidation(func(ctx context.Context, fs filesystem.FileSystem) error {
			// Validation passes
			return nil
		})

		// Create adapter
		adapter := synthfs.NewCustomOperationAdapter(op)

		// Execute directly
		err := adapter.Execute(context.Background(), fs)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}

		// Check output was stored
		status := synthfs.GetOperationOutput(adapter, "status")
		if status != "executed" {
			t.Errorf("Expected status 'executed', got %q", status)
		}
	})
}

func TestOutputCapture_RealWorldExample(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("pipeline with mixed output capture", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create test data
		testData := "apple\nbanana\ncherry\ndate\nelderberry\n"
		
		// Create pipeline with shell commands and custom operations
		ops := []synthfs.Operation{
			// Create test file
			sfs.CreateFile("fruits.txt", []byte(testData), 0644),
			
			// Count lines with shell command
			sfs.ShellCommand("wc -l < fruits.txt", 
				synthfs.WithCaptureOutput(),
				synthfs.WithWorkDir(tempDir)),
			
			// Process with custom operation
			sfs.CustomOperationWithOutput("fruit-analyzer",
				func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error {
					// Read the file
					file, err := fs.Open("fruits.txt")
					if err != nil {
						return err
					}
					defer func() {
						_ = file.Close()
					}()
					
					// Count fruits starting with specific letters
					var buf [1024]byte
					n, _ := file.Read(buf[:])
					fruits := strings.Split(strings.TrimSpace(string(buf[:n])), "\n")
					
					aCount := 0
					for _, fruit := range fruits {
						if strings.HasPrefix(fruit, "a") || strings.HasPrefix(fruit, "e") {
							aCount++
						}
					}
					
					// Store analysis results
					storeOutput("totalFruits", len(fruits))
					storeOutput("startsWithAorE", aCount)
					storeOutput("analysis", fmt.Sprintf("Found %d fruits, %d start with 'a' or 'e'", 
						len(fruits), aCount))
					
					return nil
				}),
			
			// Get first fruit with shell command
			sfs.ShellCommand("head -n 1 fruits.txt",
				synthfs.WithCaptureOutput(),
				synthfs.WithWorkDir(tempDir)),
		}

		result, err := synthfs.Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		// Check outputs from different operations
		opResults := result.GetOperations()
		
		// Check line count from wc command (operation 1)
		wcOp := opResults[1].(synthfs.OperationResult)
		lineCount := strings.TrimSpace(synthfs.GetOperationOutput(wcOp.Operation, "stdout"))
		if lineCount != "5" {
			t.Errorf("Expected line count '5', got %q", lineCount)
		}

		// Check custom operation outputs (operation 2)
		analyzerOp := opResults[2].(synthfs.OperationResult)
		totalFruits := synthfs.GetOperationOutputValue(analyzerOp.Operation, "totalFruits")
		if totalFruits != 5 {
			t.Errorf("Expected 5 total fruits, got %v", totalFruits)
		}
		
		startsWithAorE := synthfs.GetOperationOutputValue(analyzerOp.Operation, "startsWithAorE")
		if startsWithAorE != 2 { // apple and elderberry
			t.Errorf("Expected 2 fruits starting with 'a' or 'e', got %v", startsWithAorE)
		}

		analysis := synthfs.GetOperationOutput(analyzerOp.Operation, "analysis")
		expectedAnalysis := "Found 5 fruits, 2 start with 'a' or 'e'"
		if analysis != expectedAnalysis {
			t.Errorf("Expected analysis %q, got %q", expectedAnalysis, analysis)
		}

		// Check head command output (operation 3)
		headOp := opResults[3].(synthfs.OperationResult)
		firstFruit := strings.TrimSpace(synthfs.GetOperationOutput(headOp.Operation, "stdout"))
		if firstFruit != "apple" {
			t.Errorf("Expected first fruit 'apple', got %q", firstFruit)
		}
	})
}