/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

var removeContext string

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the replication target cluster from ClusterDetector CustomResource.",
	Long:  "Remove the replication target cluster from ClusterDetector CustomResource.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get path and cluster from activated file
		config, err := kubeconfig.GetPathAndCluster()
		if err != nil {
			return err
		}

		clientset, err := client.CreateClientSetFromContext(config.Path, config.Cluster)
		if err != nil {
			return err
		}

		var (
			ctx             = context.Background()
			namespaceClient = clientset.CoreV1().Namespaces()
			secretClinet    = clientset.CoreV1().Secrets(constants.KubeconfigSecretNamespace)
		)

		secret, err := secretClinet.Get(
			ctx,
			constants.KubeconfigSecretName,
			metav1.GetOptions{},
		)
		if err != nil {
			return err
		}

		modifiedContextsKubeconfig, err := kubeconfig.RemoveContext(secret, removeContext)
		if err != nil {
			return err
		}

		if err := kubeconfig.ApplyNamespacedSecret(
			ctx,
			namespaceClient,
			secretClinet,
			modifiedContextsKubeconfig,
		); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().StringVarP(&removeContext, "context", "c", "", "The context of the cluster to be removed")
}
