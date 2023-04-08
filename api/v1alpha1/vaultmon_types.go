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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VaultMonSpec defines the desired state of VaultMon
type VaultMonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	VaultName      string               `json:"name,omitempty"`
	VaultUid       string               `json:"uid,omitempty"`
	VaultNamespace string               `json:"namespace,omitempty"`
	VaultIp        string               `json:"ip,omitempty"`
	VaultLabels    map[string]string    `json:"labels,omitempty"`
	VaultSecrets   int64                `json:"secrets,omitempty"`
	VaultReplicas  int32                `json:"replicas,omitempty"`
	VaultEndpoints []string             `json:"endpoints,omitempty"`
	VaultStatus    []v1.ContainerStatus `json:"status,omitempty"`
	VaultVolumes   []string             `json:"volumes,omitempty"`
	VaultIngress   string               `json:"ingress,omitempty"`
	VaultCPUUsage  string               `json:"cpuUsage,omitempty"`
	VaultMemUsage  string               `json:"memUsage,omitempty"`
	VaultImage     string               `json:"image,omitempty"`
}

// VaultMonStatus defines the observed state of VaultMon
type VaultMonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ReplicaHealth string `json:"replicaHealth,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VaultMon is the Schema for the vaultmons API
type VaultMon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VaultMonSpec   `json:"spec,omitempty"`
	Status VaultMonStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VaultMonList contains a list of VaultMon
type VaultMonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VaultMon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VaultMon{}, &VaultMonList{})
}
