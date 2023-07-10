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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterDetectorSpec defines the desired state of ClusterDetector
type ClusterDetectorSpec struct {
	// The kubeconfig file context,cluster,user
	Context string `json:"context,omitempty"`
	Cluster string `json:"cluster,omitempty"`
	User    string `json:"user,omitempty"`
}

// ClusterDetectorStatus defines the observed state of ClusterDetector
type ClusterDetectorStatus struct {
	// If communication to the remote Kubernetes cluster is possible,
	// Running is set; if not, Unknown is set.
	ClusterStatus string `json:"clusterstatus,omitempty"`

	// An error message is output when communication with a remote Kubernetes cluster is not possible.
	// Output only when the wide option of the Kubectl get command is given.
	Reason string `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=cd
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="CONTEXT",type="string",JSONPath=".spec.context",description=
//+kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.cluster"
//+kubebuilder:printcolumn:name="USER",type="string",JSONPath=".spec.user"
//+kubebuilder:printcolumn:name="CLUSTERSTATUS",type="string",JSONPath=".status.clusterstatus"
//+kubebuilder:printcolumn:name="REASON",type="string",priority=1,JSONPath=".status.reason"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterDetector is the Schema for the clusterdetectors API
type ClusterDetector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDetectorSpec   `json:"spec,omitempty"`
	Status ClusterDetectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterDetectorList contains a list of ClusterDetector
type ClusterDetectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDetector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDetector{}, &ClusterDetectorList{})
}
