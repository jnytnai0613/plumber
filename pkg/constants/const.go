package constants

// CLI Info
const (
	ActivateDir               = ".plumber"
	AuthInfo                  = "kubernetes-admin"
	ClusterDetectorName       = "ClusterDetector"
	ClusterName               = "kubernetes"
	EndpointNamespace         = "default"
	EndpointName              = "kubernetes"
	FieldManager              = "plumberctl"
	KubeconfigSecretName      = "config"
	KubeconfigSecretNamespace = "kubeconfig"
	KubeconfigSecretKey       = "config"
	Namespace                 = "plumber-system"
	PrimaryContext            = "primary"
)

// Secret Info
const (
	IngressSecretName = "ca-secret"
	ClientSecretName  = "cli-secret"
)

// Ingress Info
const (
	IngressClassName = "nginx"
)
