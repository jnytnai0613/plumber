package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	plumberv1 "github.com/jnytnai0613/plumber/api/v1"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

func CreatePrimaryClientsets() (map[string]*kubernetes.Clientset, error) {
	clientConfig := ctrl.GetConfigOrDie()
	cs, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	clientsets := make(map[string]*kubernetes.Clientset)
	clientsets[fmt.Sprintf("%s.%s", constants.ClusterName, constants.AuthInfo)] = cs

	return clientsets, nil
}

func CreateSecondaryClientsets(
	ctx context.Context,
	cli client.Client,
	replicator plumberv1.Replicator,
) (map[string]*kubernetes.Clientset, error) {
	var secret corev1.Secret

	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: constants.KubeconfigSecretNamespace,
		Name:      constants.KubeconfigSecretName,
	}, &secret); err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}

	// Get the list of contexts (remote Kubernetes clusters) from the kubeconfig file.
	cmdConfig, err := kubeconfig.ReadKubeconfigFromByte(secret.Data[constants.KubeconfigSecretKey])
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	configPath := filepath.Join(os.TempDir(), "config")
	if err := clientcmd.WriteToFile(
		*cmdConfig,
		configPath); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig file: %w", err)
	}

	// Specify the path of the kubeconfig file to be loaded in clientcmd.ClientConfigLoadingRules.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath

	// Generate as many client sets as the number of contexts (remote Kubernetes clusters) read from kubeconfig.
	clientsets := make(map[string]*kubernetes.Clientset)
	for k, v := range cmdConfig.Contexts {
		for _, secondaryCluster := range replicator.Spec.TargetCluster {
			if fmt.Sprintf("%s.%s", v.Cluster, v.AuthInfo) != secondaryCluster {
				continue
			}
			overrides := clientcmd.ConfigOverrides{
				CurrentContext: k,
			}
			config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)
			clientConfig, err := config.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to generate client config: %w", err)
			}

			cs, err := kubernetes.NewForConfig(clientConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to generate clientset: %w", err)
			}
			clientsets[fmt.Sprintf("%s.%s", v.Cluster, v.AuthInfo)] = cs
		}
	}

	return clientsets, nil
}

// Create client for the custom resource.
func CreateLocalClient(log logr.Logger, scheme runtime.Scheme) (client.Client, *rest.Config, error) {
	clientConfig := ctrl.GetConfigOrDie()
	kubeClient, err := client.New(clientConfig, client.Options{Scheme: &scheme})
	if err != nil {
		return nil, nil, err
	}

	return kubeClient, clientConfig, nil
}

// Create a clientset for the primary cluster.
func CreateClientSetFromRestConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// Create a clientset for the secondary cluster.
func CreateClientSetFromCurrentContext(configPath string, currContext string) (*kubernetes.Clientset, error) {
	// Specify the path of the kubeconfig file to be loaded in clientcmd.ClientConfigLoadingRules.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath

	// Generate as many client sets as the number of contexts (remote Kubernetes clusters) read from kubeconfig.
	overrides := clientcmd.ConfigOverrides{
		CurrentContext: currContext,
	}
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides)
	clientConfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
