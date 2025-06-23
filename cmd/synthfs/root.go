package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "synthfs",
	Short: "A filesystem operation planning and execution tool",
	Long: `synthfs is a tool for creating, managing, and executing filesystem operation plans.
It allows you to define sequences of filesystem operations (like creating files and directories)
in a declarative way, with support for dependencies and conflict resolution.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.synthfs.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// Add version command
	rootCmd.AddCommand(versionCmd)

	// Add subcommands
	rootCmd.AddCommand(newPlanCommand())
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of synthfs`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("synthfs version %s (commit: %s, built: %s)\n", version, commit, date)
	},
}
