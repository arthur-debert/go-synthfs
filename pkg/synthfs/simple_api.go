package synthfs

import (
	"context"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Run executes a series of operations in sequence.
//
// Operations are executed in the order provided, with each operation's success required
// before proceeding. If an operation fails, subsequent operations are not executed.
// By default, this function does not perform a rollback. To enable rollback,
// use RunWithOptions with RollbackOnError set to true.
//
// Example - Simple sequential operations:
//
//	fs := synthfs.NewOSFileSystem("/tmp")
//	sfs := synthfs.New()
//	
//	result, err := synthfs.Run(ctx, fs,
//		sfs.CreateDir("project", 0755),
//		sfs.CreateFile("project/README.md", []byte("# Project"), 0644),
//		sfs.CreateFile("project/main.go", []byte("package main"), 0644),
//	)
//	
//	if err != nil {
//		log.Fatal(err)
//	}
//	log.Printf("Executed %d operations in %v", len(result.Operations), result.Duration)
//
// Example - Operations with dependencies:
//
//	result, err := synthfs.Run(ctx, fs,
//		sfs.CreateDir("data", 0755),                    // Must happen first
//		sfs.CreateFile("data/config.json", data, 0644), // Depends on directory existing
//		sfs.Copy("data/config.json", "backup.json"),    // Depends on file existing
//	)
//
// For complex dependency management, consider using BuildPipeline instead.
func Run(ctx context.Context, fs filesystem.FullFileSystem, ops ...Operation) (*Result, error) {
	return RunWithOptions(ctx, fs, DefaultPipelineOptions(), ops...)
}

// RunWithOptions executes operations with custom options.
// It builds a pipeline with the given operations and runs it using the main executor.
// This ensures that all operations are validated before execution begins.
func RunWithOptions(ctx context.Context, fs filesystem.FullFileSystem, options PipelineOptions, ops ...Operation) (*Result, error) {
	// For the simple API, we disable prerequisite resolution because the sequential
	// execution order implicitly handles dependencies.
	options.ResolvePrerequisites = false

	if options.DryRun {
		fs = NewDryRunFS()
	}

	if len(ops) == 0 {
		return &Result{
			success:    true,
			operations: []interface{}{},
			duration:   0,
		}, nil
	}

	// Build a pipeline from the operations.
	pipeline := NewMemPipeline()
	for _, op := range ops {
		if err := pipeline.Add(op); err != nil {
			// This can happen if there are duplicate operation IDs, for example.
			return nil, err
		}
	}

	// Use the main executor to run the pipeline.
	executor := NewExecutor()
	result := executor.RunWithOptions(ctx, pipeline, fs, options)

	// The executor's result is already in the desired format.
	// We just need to extract the top-level error for the return signature.
	var err error
	if !result.IsSuccess() {
		err = result.GetError()
	}

	return result, err
}
