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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VaultSpec defines the desired state of Vault
type VaultSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Vault. Edit vault_types.go to remove/update
	//Foo string `json:"foo,omitempty"`
	VaultName      string            `json:"name,omitempty"`
	VaultUid       string            `json:"uid,omitempty"`
	VaultNamespace string            `json:"namespace,omitempty"`
	VaultIp        string            `json:"ip,omitempty"`
	VaultLabels    map[string]string `json:"labels,omitempty"`
	VaultSecrets   int64             `json:"secrets,omitempty"`
	VaultReplicas  int32             `json:"replicas,omitempty"`
	VaultEndpoints []string          `json:"endpoints,omitempty"`
	VaultStatus    string            `json:"status,omitempty"`
	VaultVolumes   []string          `json:"volumes,omitempty"`
	VaultIngress   string            `json:"ingress,omitempty"`
}

// VaultStatus defines the observed state of Vault
type VaultStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Vault is the Schema for the vaults API
type Vault struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VaultSpec   `json:"spec,omitempty"`
	Status VaultStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VaultList contains a list of Vault
type VaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vault `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vault{}, &VaultList{})
}
