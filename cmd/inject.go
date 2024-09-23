// Package cmd ...
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/m198799/timezone-webhook/internal/inject"
)

var patchGenerator = inject.NewPatchGenerator()

var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "inject timezone and system out yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("you must specify at least one input")
		}

		inputs, err := inject.ArgumentsToInputs(args)
		if err != nil {
			return fmt.Errorf("failed to open inputs from arguments: %w", err)
		}

		transformer := &inject.Transformer{
			PatchGenerator: patchGenerator,
			Inputs:         inputs,
			Output:         os.Stdout,
		}

		return transformer.Transform()
	},
}

func init() {
	rootCmd.AddCommand(injectCmd)

	injectCmd.Flags().StringVarP(&patchGenerator.Timezone, "timezone", "t", patchGenerator.Timezone, "Default timezone if not specified explicitly")
	injectCmd.Flags().StringVarP((*string)(&patchGenerator.Strategy), "strategy", "s", string(patchGenerator.Strategy), "Default injection strategy if not specified explicitly (hostPath/initContainer)")
	injectCmd.Flags().StringVar(&patchGenerator.HostPathPrefix, "hostpath", patchGenerator.HostPathPrefix, "Location of TZif files on host machines")
	injectCmd.Flags().StringVarP(&patchGenerator.LocalTimePath, "mountpath", "m", patchGenerator.LocalTimePath, "Mount path for TZif file on containers")
}
