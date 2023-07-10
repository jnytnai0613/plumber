package kubeconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/jnytnai0613/plumber/pkg/constants"
)

type Config struct {
	Path    string
	Cluster string
}

var config Config

// Generate a secret resource for the kubeconfig file.
func ApplyNamespacedSecret(
	ctx context.Context,
	namespaceClient v1.NamespaceInterface,
	secretClinet v1.SecretInterface,
	config []byte,
) error {
	nsApplyConfig := corev1apply.Namespace(constants.KubeconfigSecretNamespace)
	if _, err := namespaceClient.Apply(
		ctx,
		nsApplyConfig,
		metav1.ApplyOptions{
			FieldManager: constants.FieldManager,
			Force:        true,
		},
	); err != nil {
		return err
	}

	secretApplyConfig := corev1apply.Secret(
		constants.KubeconfigSecretName,
		constants.KubeconfigSecretNamespace).
		WithData(map[string][]byte{
			constants.KubeconfigSecretKey: config,
		})

	if _, err := secretClinet.Apply(
		ctx,
		secretApplyConfig,
		metav1.ApplyOptions{
			FieldManager: constants.FieldManager,
			Force:        true,
		},
	); err != nil {
		return err
	}

	return nil
}

// Get the path and cluster name from the toml file.
func GetPathAndCluster() (Config, error) {
	// Get path and cluster from toml file
	configDir := fmt.Sprintf("%s/%s",
		os.Getenv("HOME"),
		constants.ActivateDir)
	viper.AddConfigPath(configDir)
	viper.SetConfigName(constants.KubeconfigSecretName)
	viper.SetConfigType("toml")
	if err := viper.ReadInConfig(); err != nil {
		return config, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}

// Generate a toml file that stores the path and cluster name of the kubeconfig file.
func GenerateConfigFile(path string, cluster string) error {
	// Set config file name in viper
	dir, file := filepath.Split(path)
	viper.AddConfigPath(dir)
	viper.SetConfigName(file)
	viper.SetConfigType("toml")

	// Writes the name of the specified configuration file and the context to which it is connected.
	fullpath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	viper.Set("path", fullpath)
	viper.Set("cluster", cluster)
	if err := os.Mkdir(
		fmt.Sprintf("%s/%s", os.Getenv("HOME"), constants.ActivateDir),
		os.ModePerm); err != nil {
		return err
	}
	activateFile := filepath.Join(
		fmt.Sprintf("%s/%s", os.Getenv("HOME"), constants.ActivateDir),
		fmt.Sprintf("%s.toml", file))
	if err := viper.WriteConfigAs(activateFile); err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("activate file was written out to %s", activateFile))

	return nil
}

// Generate a kubeconfig file for the primary cluster.
func GeneratePrimaryConfig(clientset *kubernetes.Clientset, restConfig *rest.Config) ([]byte, error) {
	secretClient := clientset.CoreV1().Secrets(constants.KubeconfigSecretNamespace)
	secret, err := secretClient.Get(
		context.TODO(),
		constants.KubeconfigSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	}

	// If the secret resource exists, return the kubeconfig file.
	if secret.Data[constants.KubeconfigSecretKey] != nil {
		fmt.Println("The secret resource already exists.")
		fmt.Println(string(secret.Data[constants.KubeconfigSecretKey]))
		return secret.Data[constants.KubeconfigSecretKey], nil
	}

	cmdContext := &clientcmdapi.Context{
		Cluster:  constants.ClusterName,
		AuthInfo: constants.AuthInfo,
	}
	cmdCluser := &clientcmdapi.Cluster{
		Server:                   restConfig.Host,
		CertificateAuthorityData: restConfig.CAData,
	}
	cmdAuthInfo := &clientcmdapi.AuthInfo{
		ClientCertificateData: restConfig.CertData,
		ClientKeyData:         restConfig.KeyData,
	}

	clusters := make(map[string]*clientcmdapi.Cluster)
	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	contexts := make(map[string]*clientcmdapi.Context)

	clusters[cmdContext.Cluster] = cmdCluser
	authInfos[cmdContext.AuthInfo] = cmdAuthInfo
	contexts[constants.PrimaryContext] = cmdContext

	cmdConfig := &clientcmdapi.Config{
		Clusters:  clusters,
		AuthInfos: authInfos,
		Contexts:  contexts,
	}

	primaryConfig, err := clientcmd.Write(*cmdConfig)
	if err != nil {
		return nil, err
	}

	return primaryConfig, nil
}

// Extract the kubeconfig file for the specified context.
func ExtractKubeconfig(path string, targetContext string) ([]byte, error) {
	config, err := ReadKubeconfigFromFile(path)
	if err != nil {
		return nil, err
	}

	var extractKubeconfig []byte
	for k, v := range config.Contexts {
		if k == targetContext {
			clusterName := v.Cluster
			authInfo := v.AuthInfo
			contextName := k
			apiEndpoint := string(config.Clusters[clusterName].Server)
			authData := config.Clusters[clusterName].CertificateAuthorityData
			certData := config.AuthInfos[authInfo].ClientCertificateData
			keyData := config.AuthInfos[authInfo].ClientKeyData

			cmdContext := &clientcmdapi.Context{
				Cluster:  clusterName,
				AuthInfo: authInfo,
			}
			cmdCluster := &clientcmdapi.Cluster{
				Server:                   apiEndpoint,
				CertificateAuthorityData: authData,
			}
			cmdAuthInfo := &clientcmdapi.AuthInfo{
				ClientCertificateData: certData,
				ClientKeyData:         keyData,
			}

			clusters := make(map[string]*clientcmdapi.Cluster)
			contexts := make(map[string]*clientcmdapi.Context)
			auths := make(map[string]*clientcmdapi.AuthInfo)

			// Add the cluster, authinfo, and context data to the new kubeconfig file.
			clusters[clusterName] = cmdCluster
			contexts[contextName] = cmdContext
			auths[authInfo] = cmdAuthInfo
			cmdConfig := &clientcmdapi.Config{
				Clusters:  clusters,
				Contexts:  contexts,
				AuthInfos: auths,
			}

			extractKubeconfig, err := clientcmd.Write(*cmdConfig)
			if err != nil {
				return nil, err
			}

			return extractKubeconfig, nil
		}
	}

	return extractKubeconfig, nil
}

// Read the kubeconfig file.
func ReadKubeconfigFromFile(path string) (*clientcmdapi.Config, error) {
	kubeconfigFile, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	return kubeconfigFile, nil
}

// Read the kubeconfig file.
func ReadKubeconfigFromByte(config []byte) (*clientcmdapi.Config, error) {
	kubeconfigFile, err := clientcmd.Load(config)
	if err != nil {
		return nil, err
	}

	return kubeconfigFile, nil
}

// Read the kubeconfig file registered for the Secret Resource and register the necessary information
// for apiServers and clusters, respectively. At this time, a slice element is created for
// each Kubernetes cluster. kubeconfig must be created in advance as a secret resource.
func ReadKubeconfigFromClient(cli client.Client) (*clientcmdapi.Config, error) {
	secret := &corev1.Secret{}
	if err := cli.Get(
		context.Background(),
		client.ObjectKey{
			Namespace: constants.KubeconfigSecretNamespace,
			Name:      constants.KubeconfigSecretName,
		},
		secret,
	); err != nil {
		return nil, err
	}

	m := secret.Data
	k := m[constants.KubeconfigSecretKey]
	c, err := clientcmd.Load(k)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Merge the kubeconfig.
func MargeKubeconfig(source clientcmdapi.Config, target clientcmdapi.Config) ([]byte, error) {
	// Merge the kubeconfig files.
	for k, v := range target.Clusters {
		source.Clusters[k] = v
	}
	for k, v := range target.AuthInfos {
		source.AuthInfos[k] = v
	}
	for k, v := range target.Contexts {
		source.Contexts[k] = v
	}

	result, err := clientcmd.Write(source)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func RemoveContext(config *corev1.Secret, removeTarget string) ([]byte, error) {
	m := config.Data
	k := m[constants.KubeconfigSecretKey]
	c, err := clientcmd.Load(k)
	if err != nil {
		return nil, err
	}

	for k, v := range c.Contexts {
		if k == removeTarget {
			server := v.Cluster
			authInfo := v.AuthInfo
			delete(c.Clusters, server)
			delete(c.AuthInfos, authInfo)
			delete(c.Contexts, k)
		}
	}

	result, err := clientcmd.Write(*c)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ViewContext(config *corev1.Secret) ([][]string, error) {
	m := config.Data
	k := m[constants.KubeconfigSecretKey]
	c, err := clientcmd.Load(k)
	if err != nil {
		return nil, err
	}

	data := [][]string{}
	for k, v := range c.Contexts {
		server := v.Cluster
		authInfo := v.AuthInfo
		data = append(data, []string{k, server, authInfo})
	}

	return data, nil
}
