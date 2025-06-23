package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNewRootCmd tests the initialization of the root command and its subcommands.
func TestRootCmdSetup(t *testing.T) {
	// Explicitly use cobra type to ensure import is recognized
	var _ *cobra.Command = rootCmd

	// The root command in root.go is a package variable `rootCmd`.
	// To test its initialization (which happens in init()), we can just check it.
	// Since `rootCmd` is initialized by `init()` which also adds `versionCmd`,
	// we are effectively testing that `init()` process.

	if rootCmd == nil {
		t.Fatal("rootCmd is nil after init")
	}

	expectedUse := "synthfs"
	if rootCmd.Use != expectedUse {
		t.Errorf("expected command Use %q, got %q", expectedUse, rootCmd.Use)
	}

	// Check if version subcommand is added
	foundVersionCmd := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "version" {
			foundVersionCmd = true
			break
		}
	}
	if !foundVersionCmd {
		t.Error("version subcommand not found")
	}
}

// Note: If root.go were structured with a NewRootCmd() constructor, the tests would call that.
// This approach tests the existing structure with package-level var and init().
// Test execution order can sometimes affect tests relying on init(), but `go test`
// typically handles this for a single package.
