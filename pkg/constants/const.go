/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
