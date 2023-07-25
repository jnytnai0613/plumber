/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

// viewCmd represents the view command
var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display Cluster information in table format",
	Long:  "Display Cluster information in table format",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := kubeconfig.GetPathAndCluster()
		if err != nil {
			return err
		}

		clientset, err := client.CreateClientSetFromContext(config.Path, config.Cluster)
		if err != nil {
			return err
		}

		var (
			ctx          = context.Background()
			secretClinet = clientset.CoreV1().Secrets(constants.KubeconfigSecretNamespace)
		)
		secret, err := secretClinet.Get(
			ctx,
			constants.KubeconfigSecretName,
			metav1.GetOptions{},
		)
		if err != nil {
			return err
		}

		tblData, err := kubeconfig.ViewContext(secret)
		if err != nil {
			return err
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"CONTEXT", "ClUSER", "USER"})
		for i := range tblData {
			table.Append(tblData[i])
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
