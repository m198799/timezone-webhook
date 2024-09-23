// Package cmd ...
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/m198799/timezone-webhook/internal/log"
)

var kubeConfigFile = "/Users/m/.kube/config"

var rootCmd = &cobra.Command{
	Use: "webhook",
}

// Execute ...
func Execute() {
	defer log.Flush() //nolint:errcheck
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize()

	rootCmd.PersistentFlags().StringVar(&kubeConfigFile, "kube-config", kubeConfigFile, "Path to kubeconfig file")
}
