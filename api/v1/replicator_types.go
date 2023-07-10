/*
MIT License
Copyright (c) 2023 Junya Taniai

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package v1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	networkv1apply "k8s.io/client-go/applyconfigurations/networking/v1"
)

type DeploymentSpecApplyConfiguration appsv1apply.DeploymentSpecApplyConfiguration
type ServiceSpecApplyConfiguration corev1apply.ServiceSpecApplyConfiguration
type IngressSpecApplyConfiguration networkv1apply.IngressSpecApplyConfiguration

func (c *DeploymentSpecApplyConfiguration) DeepCopy() *DeploymentSpecApplyConfiguration {
	out := new(DeploymentSpecApplyConfiguration)
	bytes, err := json.Marshal(c)
	if err != nil {
		panic("Failed to marshal")
	}
	err = json.Unmarshal(bytes, out)
	if err != nil {
		panic("Failed to unmarshal")
	}
	return out
}

func (c *ServiceSpecApplyConfiguration) DeepCopy() *ServiceSpecApplyConfiguration {
	out := new(ServiceSpecApplyConfiguration)
	bytes, err := json.Marshal(c)
	if err != nil {
		panic("Failed to marshal")
	}
	err = json.Unmarshal(bytes, out)
	if err != nil {
		panic("Failed to unmarshal")
	}
	return out
}

func (c *IngressSpecApplyConfiguration) DeepCopy() *IngressSpecApplyConfiguration {
	out := new(IngressSpecApplyConfiguration)
	bytes, err := json.Marshal(c)
	if err != nil {
		panic("Failed to marshal")
	}
	err = json.Unmarshal(bytes, out)
	if err != nil {
		panic("Failed to unmarshal")
	}
	return out
}

// ReplicatorSpec defines the desired state of Replicator
type ReplicatorSpec struct {
	ReplicationNamespace string                            `json:"replicationNamespace"`
	DeploymentName       string                            `json:"deploymentName"`
	DeploymentSpec       *DeploymentSpecApplyConfiguration `json:"deploymentSpec"`

	//+optional
	ConfigMapName string `json:"configMapName"`

	//+optional
	ConfigMapData map[string]string `json:"configMapData"`

	//+optional
	ServiceName string `json:"serviceName"`

	//+optional
	ServiceSpec *ServiceSpecApplyConfiguration `json:"serviceSpec"`

	//+optional
	IngressName string `json:"ingressName"`

	//+optional
	IngressSpec *IngressSpecApplyConfiguration `json:"ingressSpec"`

	//+optional
	IngressSecureEnabled bool `json:"ingressSecureEnabled"`

	//+optional
	TargetCluster []string `json:"targetCluster"`
}

// ReplicatorStatus defines the observed state of Replicator
type ReplicatorStatus struct {
	// Synchronization status with remote Kubernetes cluster per resource
	Applied []PerResourceApplyStatus `json:"applied"`

	// The status will be as follows
	// synced: Resource Apply succeeded on all clusters
	// not synced: Resource Apply failed in any of the clusters.
	Synced string `json:"synced"`
}

type PerResourceApplyStatus struct {
	Cluster     string `json:"cluster"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	ApplyStatus string `json:"applyStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster,shortName=rep
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.synced"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Replicator is the Schema for the replicators API
type Replicator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicatorSpec   `json:"spec,omitempty"`
	Status ReplicatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReplicatorList contains a list of Replicator
type ReplicatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Replicator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Replicator{}, &ReplicatorList{})
}
