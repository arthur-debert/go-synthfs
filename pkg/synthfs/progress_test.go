package synthfs_test

import (
	"context"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/ops"
)

// Mock progress reporter for testing
type mockProgressReporter struct {
	onStartCalls    []synthfs.Operation
	onProgressCalls []progressCall
	onCompleteCalls []synthfs.OperationResult
	onFinishCalls   []finishCall
}

type progressCall struct {
	op      synthfs.Operation
	current int64
	total   int64
}

type finishCall struct {
	totalOps     int
	successCount int
	duration     time.Duration
}

func newMockProgressReporter() *mockProgressReporter {
	return &mockProgressReporter{
		onStartCalls:    []synthfs.Operation{},
		onProgressCalls: []progressCall{},
		onCompleteCalls: []synthfs.OperationResult{},
		onFinishCalls:   []finishCall{},
	}
}

func (m *mockProgressReporter) OnStart(op synthfs.Operation) {
	m.onStartCalls = append(m.onStartCalls, op)
}

func (m *mockProgressReporter) OnProgress(op synthfs.Operation, current, total int64) {
	m.onProgressCalls = append(m.onProgressCalls, progressCall{op, current, total})
}

func (m *mockProgressReporter) OnComplete(op synthfs.Operation, result synthfs.OperationResult) {
	m.onCompleteCalls = append(m.onCompleteCalls, result)
}

func (m *mockProgressReporter) OnFinish(totalOps int, successCount int, duration time.Duration) {
	m.onFinishCalls = append(m.onFinishCalls, finishCall{totalOps, successCount, duration})
}

func TestProgressReportingExecutor(t *testing.T) {
	t.Run("NewProgressReportingExecutor", func(t *testing.T) {
		reporter := newMockProgressReporter()
		executor := synthfs.NewProgressReportingExecutor(reporter)

		if executor == nil {
			t.Errorf("Expected non-nil executor")
		}
	})

	t.Run("ExecuteWithProgress - successful operations", func(t *testing.T) {
		reporter := newMockProgressReporter()
		executor := synthfs.NewProgressReportingExecutor(reporter)
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Create a queue with two operations
		queue := synthfs.NewMemQueue()
		op1 := ops.NewCreateFile("file1.txt", []byte("content1"), 0644)
		op2 := ops.NewCreateFile("file2.txt", []byte("content2"), 0644)
		queue.Add(op1, op2)

		// Execute with progress
		result := executor.ExecuteWithProgress(ctx, queue, tfs)

		// Check result
		if !result.Success {
			t.Errorf("Expected successful execution, got errors: %v", result.Errors)
		}

		if len(result.Operations) != 2 {
			t.Errorf("Expected 2 operation results, got %d", len(result.Operations))
		}

		// Check progress reporting calls
		if len(reporter.onStartCalls) != 2 {
			t.Errorf("Expected 2 OnStart calls, got %d", len(reporter.onStartCalls))
		}

		if len(reporter.onProgressCalls) != 2 {
			t.Errorf("Expected 2 OnProgress calls, got %d", len(reporter.onProgressCalls))
		}

		if len(reporter.onCompleteCalls) != 2 {
			t.Errorf("Expected 2 OnComplete calls, got %d", len(reporter.onCompleteCalls))
		}

		if len(reporter.onFinishCalls) != 1 {
			t.Errorf("Expected 1 OnFinish call, got %d", len(reporter.onFinishCalls))
		}

		// Check progress call details
		for i, call := range reporter.onProgressCalls {
			if call.current != int64(i+1) {
				t.Errorf("Expected progress current %d, got %d", i+1, call.current)
			}
			if call.total != 2 {
				t.Errorf("Expected progress total 2, got %d", call.total)
			}
		}

		// Check finish call
		finishCall := reporter.onFinishCalls[0]
		if finishCall.totalOps != 2 {
			t.Errorf("Expected total ops 2, got %d", finishCall.totalOps)
		}
		if finishCall.successCount != 2 {
			t.Errorf("Expected success count 2, got %d", finishCall.successCount)
		}
	})

	t.Run("ExecuteWithProgress - failed operation", func(t *testing.T) {
		reporter := newMockProgressReporter()
		executor := synthfs.NewProgressReportingExecutor(reporter)
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Create a file first, then try to create a directory with the same name
		// This will fail during execution, not validation
		err := tfs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		// Create a queue with an operation that will fail during execution
		queue := synthfs.NewMemQueue()
		op := ops.NewCreateDir("conflict.txt", 0755) // Try to create dir where file exists
		queue.Add(op)

		// Execute with progress
		result := executor.ExecuteWithProgress(ctx, queue, tfs)

		// Check result
		if result.Success {
			t.Errorf("Expected failed execution")
		}

		// Check progress reporting calls
		if len(reporter.onStartCalls) != 1 {
			t.Errorf("Expected 1 OnStart call, got %d", len(reporter.onStartCalls))
		}

		if len(reporter.onCompleteCalls) != 1 {
			t.Errorf("Expected 1 OnComplete call, got %d", len(reporter.onCompleteCalls))
		}

		// Check that the operation result indicates failure
		opResult := reporter.onCompleteCalls[0]
		if opResult.Status == synthfs.StatusSuccess {
			t.Errorf("Expected operation to fail")
		}
	})

	t.Run("ExecuteWithProgress - dry run", func(t *testing.T) {
		reporter := newMockProgressReporter()
		executor := synthfs.NewProgressReportingExecutor(reporter)
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Create a queue with one operation
		queue := synthfs.NewMemQueue()
		op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
		queue.Add(op)

		// Execute with dry run
		result := executor.ExecuteWithProgress(ctx, queue, tfs, synthfs.WithDryRun(true))

		// Check result - should be successful but skipped
		if !result.Success {
			t.Errorf("Expected successful execution for dry run")
		}

		// Check that the operation was skipped
		opResult := reporter.onCompleteCalls[0]
		if opResult.Status != synthfs.StatusSkipped {
			t.Errorf("Expected operation to be skipped, got %s", opResult.Status)
		}

		// File should not exist
		_, err := tfs.Stat("test.txt")
		if err == nil {
			t.Errorf("Expected file to not exist in dry run")
		}
	})
}

func TestExecuteStream(t *testing.T) {
	t.Run("Successful execution", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Create a queue
		queue := synthfs.NewMemQueue()
		op1 := ops.NewCreateFile("file1.txt", []byte("content1"), 0644)
		op2 := ops.NewCreateFile("file2.txt", []byte("content2"), 0644)
		queue.Add(op1, op2)

		// Execute with streaming
		resultChan := executor.ExecuteStream(ctx, queue, tfs)

		// Collect results
		var results []synthfs.OperationResult
		for result := range resultChan {
			results = append(results, result)
		}

		// Check results
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		for _, result := range results {
			if result.Status != synthfs.StatusSuccess {
				t.Errorf("Expected successful status, got %s: %v", result.Status, result.Error)
			}
		}
	})

	t.Run("Failed validation", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Create a file first, then try to create a directory with the same name
		err := tfs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		// Create a queue with operation that will fail during execution
		queue := synthfs.NewMemQueue()
		op := ops.NewCreateDir("conflict.txt", 0755) // Try to create dir where file exists
		queue.Add(op)

		// Execute with streaming
		resultChan := executor.ExecuteStream(ctx, queue, tfs)

		// Collect results
		var results []synthfs.OperationResult
		for result := range resultChan {
			results = append(results, result)
		}

		// Check results
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if results[0].Status == synthfs.StatusSuccess {
			t.Errorf("Expected operation to fail")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		tfs := synthfs.NewTestFileSystem()
		ctx, cancel := context.WithCancel(context.Background())

		// Create a queue
		queue := synthfs.NewMemQueue()
		op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
		queue.Add(op)

		// Cancel context immediately
		cancel()

		// Execute with streaming
		resultChan := executor.ExecuteStream(ctx, queue, tfs)

		// Collect results
		var results []synthfs.OperationResult
		for result := range resultChan {
			results = append(results, result)
		}

		// Should get cancelled operation
		if len(results) != 1 {
			t.Errorf("Expected 1 result for cancelled context, got %d", len(results))
		}

		if results[0].Error != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", results[0].Error)
		}
	})
}

func TestConsoleProgressReporter(t *testing.T) {
	t.Run("NewConsoleProgressReporter", func(t *testing.T) {
		reporter := synthfs.NewConsoleProgressReporter(true)
		if reporter == nil {
			t.Errorf("Expected non-nil reporter")
		}

		reporter = synthfs.NewConsoleProgressReporter(false)
		if reporter == nil {
			t.Errorf("Expected non-nil reporter")
		}
	})

	// Note: We can't easily test the actual console output without capturing stdout,
	// but we can at least verify the methods don't panic
	t.Run("Method calls don't panic", func(t *testing.T) {
		reporter := synthfs.NewConsoleProgressReporter(true)
		op := ops.NewCreateFile("test.txt", []byte("content"), 0644)

		// These should not panic
		reporter.OnStart(op)
		reporter.OnProgress(op, 1, 2)

		result := synthfs.OperationResult{
			OperationID: op.ID(),
			Operation:   op,
			Status:      synthfs.StatusSuccess,
			Duration:    time.Millisecond,
		}
		reporter.OnComplete(op, result)
		reporter.OnFinish(2, 2, time.Millisecond)
	})
}

func TestNoOpProgressReporter(t *testing.T) {
	t.Run("NewNoOpProgressReporter", func(t *testing.T) {
		reporter := synthfs.NewNoOpProgressReporter()
		if reporter == nil {
			t.Errorf("Expected non-nil reporter")
		}
	})

	t.Run("Method calls don't panic", func(t *testing.T) {
		reporter := synthfs.NewNoOpProgressReporter()
		op := ops.NewCreateFile("test.txt", []byte("content"), 0644)

		// These should not panic
		reporter.OnStart(op)
		reporter.OnProgress(op, 1, 2)

		result := synthfs.OperationResult{
			OperationID: op.ID(),
			Operation:   op,
			Status:      synthfs.StatusSuccess,
			Duration:    time.Millisecond,
		}
		reporter.OnComplete(op, result)
		reporter.OnFinish(2, 2, time.Millisecond)
	})
}

func TestProgressReporting_Integration(t *testing.T) {
	// Integration test with real operations and progress reporting
	reporter := newMockProgressReporter()
	executor := synthfs.NewProgressReportingExecutor(reporter)
	tfs := synthfs.NewTestFileSystem()
	ctx := context.Background()

	// Create a complex queue with dependencies
	queue := synthfs.NewMemQueue()

	// Create directory first
	createDirOp := ops.NewCreateDir("testdir", 0755).WithID("create-dir")

	// Create file that depends on directory
	createFileOp := ops.NewCreateFile("testdir/file.txt", []byte("content"), 0644).
		WithID("create-file").
		WithDependency("create-dir")

	queue.Add(createDirOp, createFileOp)

	// Execute
	result := executor.ExecuteWithProgress(ctx, queue, tfs)

	// Verify execution
	if !result.Success {
		t.Fatalf("Expected successful execution, got errors: %v", result.Errors)
	}

	// Verify files were created
	_, err := tfs.Stat("testdir")
	if err != nil {
		t.Errorf("Expected directory to exist: %v", err)
	}

	_, err = tfs.Stat("testdir/file.txt")
	if err != nil {
		t.Errorf("Expected file to exist: %v", err)
	}

	// Verify progress reporting
	if len(reporter.onStartCalls) != 2 {
		t.Errorf("Expected 2 OnStart calls, got %d", len(reporter.onStartCalls))
	}

	if len(reporter.onCompleteCalls) != 2 {
		t.Errorf("Expected 2 OnComplete calls, got %d", len(reporter.onCompleteCalls))
	}

	// Verify operations completed successfully
	for _, result := range reporter.onCompleteCalls {
		if result.Status != synthfs.StatusSuccess {
			t.Errorf("Expected operation to succeed, got %s: %v", result.Status, result.Error)
		}
	}
}
