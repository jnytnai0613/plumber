/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

var targetContext string

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add the replication target cluster to ClusterDetector CustomResource.",
	Long:  "Add the replication target cluster to ClusterDetector CustomResource.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get path and cluster from activated file
		config, err := kubeconfig.GetPathAndCluster()
		if err != nil {
			return err
		}

		clientset, err := client.CreateClientSetFromCurrentContext(config.Path, config.Cluster)
		if err != nil {
			return err
		}

		var (
			ctx             = context.Background()
			namespaceClient = clientset.CoreV1().Namespaces()
			secretClinet    = clientset.CoreV1().Secrets(constants.KubeconfigSecretNamespace)
		)

		newKubeconfig, err := kubeconfig.ExtractKubeconfig(config.Path, targetContext)
		if err != nil {
			return err
		}

		secret, err := secretClinet.Get(
			ctx,
			constants.KubeconfigSecretName,
			metav1.GetOptions{},
		)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}

			if err := kubeconfig.ApplyNamespacedSecret(
				ctx,
				namespaceClient,
				secretClinet,
				newKubeconfig,
			); err != nil {
				return err
			}

			return nil
		}

		sourceKubeconfig, err := clientcmd.Load(secret.Data[constants.KubeconfigSecretKey])
		if err != nil {
			return err
		}
		targetKubeconfig, err := clientcmd.Load(newKubeconfig)
		if err != nil {
			return err
		}
		margeKubeconfig, err := kubeconfig.MargeKubeconfig(*targetKubeconfig, *sourceKubeconfig)
		if err != nil {
			return err
		}

		if err := kubeconfig.ApplyNamespacedSecret(
			ctx,
			namespaceClient,
			secretClinet,
			margeKubeconfig,
		); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&targetContext, "target-context", "c", "", `Cluster to REPLICATE.
It is added to Operatror's ClusterDetector resource.`)
}
