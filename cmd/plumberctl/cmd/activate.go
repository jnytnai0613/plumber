/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

var (
	path            string
	activateContext string
)

// checkConnection checks the connection to the cluster where the Operator resides.
func checkConnection(clientset *kubernetes.Clientset) error {
	var namespaceClinet = clientset.CoreV1().Namespaces()

	if _, err := namespaceClinet.Get(
		context.Background(),
		constants.Namespace,
		metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use: "activate",
	Short: `Perform a connection test to the cluster where the Operator resides 
              and write the connection information to a file.`,
	Long: `Perform a connection test to the cluster where the Operator resides
             and write the connection information to a file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		clientset, err := client.CreateClientSetFromCurrentContext(path, activateContext)
		if err != nil {
			return err
		}

		if err := checkConnection(clientset); err != nil {
			return err
		}

		if err := kubeconfig.GenerateConfigFile(path, activateContext); err != nil {
			return err
		}

		fmt.Println("Successfully activated to the cluster.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(activateCmd)
	activateCmd.Flags().StringVarP(&path, "path", "p", "", "Path to kubeconfig file")
	activateCmd.Flags().StringVarP(&activateContext, "activate-context", "a", "", "Context to be replicated")
}
