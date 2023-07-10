/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "plumberctl",
	Short: "plumberctl reads kubeconfig and generates a kubeconfig containing only the config to be replicated.",
	Long:  "plumberctl reads kubeconfig and generates a kubeconfig containing only the config to be replicated.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("The any option are required.")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
