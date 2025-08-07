package synthfs_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestShellCommand_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("execute simple command", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create a test file
		testFile := "test.txt"
		err := fs.WriteFile(testFile, []byte("hello"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Execute shell command to check file exists
		// Use working directory where files are created
		op := sfs.ShellCommand("ls test.txt", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}
	})

	t.Run("command with working directory", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create directory structure
		err := fs.MkdirAll("subdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Create file in subdirectory
		err = fs.WriteFile("subdir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Execute command in subdirectory
		subdirPath := filepath.Join(tempDir, "subdir")
		op := sfs.ShellCommand("pwd > pwd.txt", synthfs.WithWorkDir(subdirPath))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}

		// Verify pwd.txt was created in subdir
		if _, err := fs.Stat("subdir/pwd.txt"); err != nil {
			t.Error("pwd.txt was not created in subdir")
		}
	})

	t.Run("command with environment variables", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Execute command that uses environment variable
		op := sfs.ShellCommand("echo $TEST_VAR > env.txt", 
			synthfs.WithEnv(map[string]string{"TEST_VAR": "test_value"}),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}

		// Read the output file
		file, err := fs.Open("env.txt")
		if err != nil {
			t.Fatalf("Failed to open env.txt: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()

		var buf [1024]byte
		n, err := file.Read(buf[:])
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		output := strings.TrimSpace(string(buf[:n]))
		if output != "test_value" {
			t.Errorf("Expected 'test_value', got %q", output)
		}
	})

	t.Run("command with timeout", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Command that would take too long
		op := sfs.ShellCommand("sleep 5", 
			synthfs.WithTimeout(100*time.Millisecond),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err == nil {
			t.Error("Expected timeout error")
		}

		if result.IsSuccess() {
			t.Error("Command should have failed due to timeout")
		}
	})
}

func TestShellCommand_ErrorHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("command fails with error", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Command that will fail
		op := sfs.ShellCommand("ls /nonexistent/path", 
			synthfs.WithCaptureOutput(),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err == nil {
			t.Error("Expected error from failed command")
		}

		if result.IsSuccess() {
			t.Error("Command should have failed")
		}
	})

	t.Run("command with rollback", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Command that creates a file with rollback that removes it
		createOp := sfs.ShellCommand("touch rollback-test.txt",
			synthfs.WithRollbackCommand("rm -f rollback-test.txt"),
			synthfs.WithWorkDir(tempDir))

		// Command that will fail
		failOp := sfs.ShellCommand("exit 1", synthfs.WithWorkDir(tempDir))

		// Run with rollback enabled
		opts := synthfs.DefaultPipelineOptions()
		opts.RollbackOnError = true

		result, err := synthfs.RunWithOptions(context.Background(), fs, opts,
			createOp,
			failOp,
		)

		if err == nil {
			t.Error("Expected error from failed command")
		}

		if result.IsSuccess() {
			t.Error("Pipeline should have failed")
		}

		// Verify rollback was executed (file should not exist)
		if _, err := fs.Stat("rollback-test.txt"); !os.IsNotExist(err) {
			t.Error("Rollback command should have removed the file")
		}
	})
}

func TestShellCommand_InPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("shell command with filesystem operations", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Mixed pipeline
		result, err := synthfs.Run(context.Background(), fs,
			sfs.CreateDir("data", 0755),
			sfs.CreateFile("data/input.txt", []byte("hello world"), 0644),
			sfs.ShellCommand("cat data/input.txt | wc -w > data/count.txt", synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("echo 'Processing complete' > data/status.txt", synthfs.WithWorkDir(tempDir)),
		)

		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Pipeline did not succeed: %v", result.GetError())
		}

		// Verify all files were created
		files := []string{"data/input.txt", "data/count.txt", "data/status.txt"}
		for _, file := range files {
			if _, err := fs.Stat(file); err != nil {
				t.Errorf("File %s was not created", file)
			}
		}
	})

	t.Run("shell command with dependencies", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create operations
		createScript := sfs.CreateFile("script.sh", []byte("#!/bin/sh\necho 'Hello from script'"), 0755)
		makeExecutable := sfs.ShellCommand("chmod +x script.sh", synthfs.WithWorkDir(tempDir))
		runScript := sfs.ShellCommand("./script.sh > output.txt", synthfs.WithWorkDir(tempDir))

		// Build pipeline with dependencies
		builder := synthfs.NewPipelineBuilder()
		builder.Add(createScript)
		makeExecutable.AddDependency(createScript.ID())
		builder.Add(makeExecutable)
		runScript.AddDependency(makeExecutable.ID())
		builder.Add(runScript)

		pipeline := builder.Build()

		executor := synthfs.NewExecutor()
		result := executor.Run(context.Background(), pipeline, fs)

		if !result.IsSuccess() {
			t.Errorf("Pipeline failed: %v", result.GetError())
		}

		// Verify output was created
		if _, err := fs.Stat("output.txt"); err != nil {
			t.Error("Script output was not created")
		}
	})
}

func TestShellCommand_RealWorldExamples(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("build and test workflow", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create a simple Go project
		goMod := `module example.com/test
go 1.21`
		
		goMain := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func Add(a, b int) int {
	return a + b
}`

		goTest := `package main

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Add(2, 3) = %d; want 5", result)
	}
}`

		// Create build workflow
		ops := []synthfs.Operation{
			sfs.CreateFile("go.mod", []byte(goMod), 0644),
			sfs.CreateFile("main.go", []byte(goMain), 0644),
			sfs.CreateFile("main_test.go", []byte(goTest), 0644),
			sfs.ShellCommand("go mod tidy", synthfs.WithTimeout(30*time.Second), synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("go test -v", synthfs.WithTimeout(30*time.Second), synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("go build -o test-app", synthfs.WithTimeout(30*time.Second), synthfs.WithWorkDir(tempDir)),
		}

		result, err := synthfs.Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Build workflow failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Build workflow did not succeed: %v", result.GetError())
		}

		// Verify binary was created
		binaryPath := filepath.Join(tempDir, "test-app")
		if _, err := os.Stat(binaryPath); err != nil {
			t.Error("Binary was not created")
		}
	})

	t.Run("git workflow simulation", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Check if git is available
		checkGit := sfs.ShellCommand("which git", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir))
		result, _ := synthfs.Run(context.Background(), fs, checkGit)
		
		if !result.IsSuccess() {
			t.Skip("Git not available, skipping git workflow test")
		}

		// Create git workflow
		ops := []synthfs.Operation{
			sfs.ShellCommand("git init", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir)),
			sfs.CreateFile("README.md", []byte("# Test Project"), 0644),
			sfs.CreateFile(".gitignore", []byte("*.tmp\n*.log"), 0644),
			sfs.ShellCommand("git add .", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("git config user.email 'test@example.com'", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("git config user.name 'Test User'", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("git commit -m 'Initial commit'", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir)),
		}

		result, err := synthfs.Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Git workflow failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Git workflow did not succeed: %v", result.GetError())
		}

		// Verify git repo was initialized
		if _, err := fs.Stat(".git"); err != nil {
			t.Error("Git repository was not initialized")
		}
	})

	t.Run("data processing pipeline", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create sample data
		csvData := `name,age,city
Alice,30,New York
Bob,25,San Francisco
Charlie,35,Chicago
David,28,Boston
Eve,32,Seattle
`

		// Data processing pipeline
		ops := []synthfs.Operation{
			sfs.CreateDir("data", 0755),
			sfs.CreateFile("data/input.csv", []byte(csvData), 0644),
			sfs.ShellCommand("head -n 1 data/input.csv > data/header.txt", synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("tail -n +2 data/input.csv | sort -t',' -k2 -n > data/sorted_by_age.csv", synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("grep ',3[0-9],' data/input.csv > data/over_30.csv", synthfs.WithWorkDir(tempDir)),
			sfs.ShellCommand("wc -l < data/input.csv > data/record_count.txt", synthfs.WithWorkDir(tempDir)),
		}

		result, err := synthfs.Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Data processing failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Data processing did not succeed: %v", result.GetError())
		}

		// Verify outputs
		outputs := []string{
			"data/header.txt",
			"data/sorted_by_age.csv", 
			"data/over_30.csv",
			"data/record_count.txt",
		}
		
		for _, output := range outputs {
			if _, err := fs.Stat(output); err != nil {
				t.Errorf("Output file %s was not created", output)
			}
		}

		// Verify record count
		file, err := fs.Open("data/record_count.txt")
		if err != nil {
			t.Fatalf("Failed to open record count: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()

		var buf [10]byte
		n, err := file.Read(buf[:])
		if err != nil {
			t.Fatalf("Failed to read record count: %v", err)
		}

		count := strings.TrimSpace(string(buf[:n]))
		// The CSV has 6 lines total (1 header + 5 data)
		if count != "6" {
			t.Errorf("Expected record count of 6, got %s", count)
		}
	})
}

func TestShellCommand_CustomShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("use bash shell", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Check if bash is available
		checkBash := sfs.ShellCommand("which bash", synthfs.WithCaptureOutput(), synthfs.WithWorkDir(tempDir))
		result, _ := synthfs.Run(context.Background(), fs, checkBash)
		
		if !result.IsSuccess() {
			t.Skip("Bash not available, skipping bash-specific test")
		}

		// Use bash-specific syntax
		op := sfs.ShellCommand("echo ${BASH_VERSION:-not bash} > shell.txt",
			synthfs.WithShell("bash", "-c"),
			synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}

		// Read output
		file, err := fs.Open("shell.txt")
		if err != nil {
			t.Fatalf("Failed to open shell.txt: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()

		var buf [1024]byte
		n, err := file.Read(buf[:])
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		output := strings.TrimSpace(string(buf[:n]))
		if output == "not bash" {
			t.Error("Command was not executed with bash")
		}
	})
}

func TestShellCommand_EdgeCases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell command tests are Unix-specific")
	}

	t.Run("empty command", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		op := sfs.ShellCommand("", synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		// Empty command should succeed (shell will just return)
		if err != nil {
			t.Errorf("Empty command should succeed: %v", err)
		}

		if !result.IsSuccess() {
			t.Error("Empty command should succeed")
		}
	})

	t.Run("command with special characters", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Command with quotes and special chars
		op := sfs.ShellCommand(`echo "Hello 'world' $USER" > special.txt`, synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}

		// Verify file was created
		if _, err := fs.Stat("special.txt"); err != nil {
			t.Error("Output file was not created")
		}
	})

	t.Run("very long command", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Generate a long command
		var parts []string
		for i := 0; i < 100; i++ {
			parts = append(parts, fmt.Sprintf("echo %d", i))
		}
		longCommand := strings.Join(parts, " && ") + " > long.txt"

		op := sfs.ShellCommand(longCommand, synthfs.WithWorkDir(tempDir))
		
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Command did not succeed: %v", result.GetError())
		}

		// Verify output file was created
		if _, err := fs.Stat("long.txt"); err != nil {
			t.Error("Output file was not created")
		}
	})
}