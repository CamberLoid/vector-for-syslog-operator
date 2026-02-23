/*
Copyright 2026.

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

// SocketMode defines the socket mode type (tcp or udp)
type SocketMode string

const (
	// TCPMode uses TCP protocol
	TCPMode SocketMode = "tcp"
	// UDPMode uses UDP protocol
	UDPMode SocketMode = "udp"
)

// VectorSocketSourceSpec defines the desired state of VectorSocketSource
type VectorSocketSourceSpec struct {
	// Mode is the socket mode (tcp or udp)
	// This corresponds to Vector's socket source 'mode' parameter
	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default=tcp
	// +kubebuilder:validation:Required
	Mode SocketMode `json:"mode"`

	// Port is the listening port number
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	Port int32 `json:"port"`

	// NodePort is the optional NodePort to use when Service type is NodePort
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`

	// Labels to inject into log entries from this source
	// These will be added as fields in the enrich transform
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// VectorSocketSourceStatus defines the observed state of VectorSocketSource
type VectorSocketSourceStatus struct {
	// Conditions represent the current state of the VectorSocketSource resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.spec.port`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VectorSocketSource is the Schema for the vectorsocketsources API
type VectorSocketSource struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec VectorSocketSourceSpec `json:"spec"`

	// +optional
	Status VectorSocketSourceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VectorSocketSourceList contains a list of VectorSocketSource
type VectorSocketSourceList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []VectorSocketSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VectorSocketSource{}, &VectorSocketSourceList{})
}
