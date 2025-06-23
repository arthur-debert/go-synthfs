package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/spf13/cobra"
)

func newPlanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage operation plans",
		Long:  "Create, execute, and manage serialized operation plans",
	}

	cmd.AddCommand(newPlanExecuteCommand())
	cmd.AddCommand(newPlanCreateCommand())
	cmd.AddCommand(newPlanValidateCommand())

	return cmd
}

func newPlanExecuteCommand() *cobra.Command {
	var (
		dryRun bool
		root   string
	)

	cmd := &cobra.Command{
		Use:   "execute [plan-file]",
		Short: "Execute an operation plan",
		Long:  "Execute operations from a serialized plan file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planFile := args[0]

			// Read plan file
			data, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read plan file %s: %w", planFile, err)
			}

			// Unmarshal plan
			plan, err := synthfs.UnmarshalPlan(data)
			if err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			// Set up filesystem
			var fsys synthfs.FileSystem
			if root == "" {
				root = "."
			}
			fsys = synthfs.NewOSFileSystem(root)

			// Create queue from plan
			queue := plan.ToQueue()

			// Execute
			executor := synthfs.NewExecutor()
			ctx := context.Background()

			var result *synthfs.Result
			if dryRun {
				result = executor.Execute(ctx, queue, fsys, synthfs.WithDryRun(true))
				fmt.Printf("DRY RUN: Plan '%s' execution summary:\n", plan.Metadata.Description)
			} else {
				result = executor.Execute(ctx, queue, fsys)
				fmt.Printf("Plan '%s' execution summary:\n", plan.Metadata.Description)
			}

			// Print results
			for _, opResult := range result.Operations {
				status := "✓"
				if opResult.Status != synthfs.StatusSuccess {
					status = "✗"
				}
				fmt.Printf("  %s %s (%s) - %v\n", status, opResult.OperationID, opResult.Status, opResult.Duration)
				if opResult.Error != nil {
					fmt.Printf("    Error: %v\n", opResult.Error)
				}
			}

			if result.Success {
				fmt.Printf("\n✓ Plan executed successfully in %v\n", result.Duration)
				return nil
			} else {
				fmt.Printf("\n✗ Plan execution failed in %v\n", result.Duration)
				fmt.Printf("Errors:\n")
				for _, err := range result.Errors {
					fmt.Printf("  - %v\n", err)
				}
				return fmt.Errorf("plan execution failed")
			}
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate execution without making changes")
	cmd.Flags().StringVar(&root, "root", "", "Root directory for filesystem operations (default: current directory)")

	return cmd
}

func newPlanCreateCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "create [description]",
		Short: "Create a new operation plan",
		Long:  "Create a new operation plan template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			description := args[0]

			// Create a sample plan
			plan := synthfs.NewOperationPlan(description)

			// Add a sample operation
			sampleOp := synthfs.NewSerializableCreateFile("example.txt", []byte("Hello, World!"), 0644)
			plan.AddOperation(sampleOp)

			// Marshal to JSON
			data, err := synthfs.MarshalPlan(plan)
			if err != nil {
				return fmt.Errorf("failed to marshal plan: %w", err)
			}

			// Write to file
			if output == "" {
				output = "plan.json"
			}

			if err := os.WriteFile(output, data, 0644); err != nil {
				return fmt.Errorf("failed to write plan file %s: %w", output, err)
			}

			fmt.Printf("Created plan file: %s\n", output)
			fmt.Printf("Description: %s\n", description)
			fmt.Printf("Operations: %d\n", len(plan.Operations))

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output plan file (default: plan.json)")

	return cmd
}

func newPlanValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [plan-file]",
		Short: "Validate an operation plan",
		Long:  "Validate the syntax and structure of an operation plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planFile := args[0]

			// Read plan file
			data, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read plan file %s: %w", planFile, err)
			}

			// Try to unmarshal as a basic JSON first
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				return fmt.Errorf("invalid JSON in plan file: %w", err)
			}

			// Unmarshal plan
			plan, err := synthfs.UnmarshalPlan(data)
			if err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			// Create queue and validate
			queue := plan.ToQueue()

			// Validate dependencies (without filesystem context)
			if err := queue.Resolve(); err != nil {
				return fmt.Errorf("plan validation failed: %w", err)
			}

			fmt.Printf("✓ Plan file is valid\n")
			fmt.Printf("Description: %s\n", plan.Metadata.Description)
			fmt.Printf("Version: %s\n", plan.Metadata.Version)
			fmt.Printf("Operations: %d\n", len(plan.Operations))

			// Show operation summary
			for i, op := range plan.Operations {
				desc := op.Describe()
				fmt.Printf("  %d. %s: %s (%s)\n", i+1, op.ID(), desc.Path, desc.Type)
				deps := op.Dependencies()
				if len(deps) > 0 {
					fmt.Printf("     Dependencies: %v\n", deps)
				}
			}

			return nil
		},
	}

	return cmd
}
