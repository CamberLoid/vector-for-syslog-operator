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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ServiceType defines the type of Service to create
type ServiceType string

const (
	// LoadBalancerService uses LoadBalancer service type
	LoadBalancerService ServiceType = "LoadBalancer"
	// NodePortService uses NodePort service type
	NodePortService ServiceType = "NodePort"
)

// ServiceSpec defines the Service configuration
type ServiceSpec struct {
	// Type is the Service type
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort
	// +kubebuilder:default=LoadBalancer
	// +optional
	Type ServiceType `json:"type,omitempty"`

	// LoadBalancerIP is the IP address to assign to the LoadBalancer
	// Only valid for ServiceType LoadBalancer
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`

	// LoadBalancerClass is the class of the load balancer
	// Only valid for ServiceType LoadBalancer
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// Annotations to add to the Service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GlobalPipelineSpec defines global pipeline configuration
// This follows Vector's configuration file structure
type GlobalPipelineSpec struct {
	// Transforms are global transforms defined by user
	// Key is transform name, value is transform configuration
	// Use $$VectorForSyslogOperatorSources$$ as placeholder for inputs
	// Example:
	//   my_transform:
	//     type: remap
	//     inputs: $$VectorForSyslogOperatorSources$$
	//     source: '.added_field = "value"'
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Transforms map[string]runtime.RawExtension `json:"transforms,omitempty"`

	// Sinks are sinks defined by user
	// At least one sink is required
	// Key is sink name, value is sink configuration
	// Example:
	//   my_sink:
	//     type: console
	//     inputs: ["my_transform"]
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Sinks map[string]runtime.RawExtension `json:"sinks"`

	// EnrichEnabled controls whether to enable automatic enrich transforms
	// for VectorSocketSource labels. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	EnrichEnabled *bool `json:"enrichEnabled,omitempty"`
}

// VectorSyslogConfigurationSpec defines the desired state of VectorSyslogConfiguration
type VectorSyslogConfigurationSpec struct {
	// Service is the Service configuration
	// +optional
	Service ServiceSpec `json:"service,omitempty"`

	// SourceSelector selects VectorSocketSource resources to include
	// If empty, all VectorSocketSources in the namespace are selected
	// +optional
	SourceSelector *metav1.LabelSelector `json:"sourceSelector,omitempty"`

	// GlobalPipeline defines global pipeline configuration
	// including transforms and sinks
	// This follows Vector's configuration structure:
	//   transforms:
	//     <name>:
	//       type: remap
	//       inputs: $$VectorForSyslogOperatorSources$$
	//   sinks:
	//     <name>:
	//       type: console
	//       inputs: [<transform_name>]
	// +kubebuilder:validation:Required
	GlobalPipeline GlobalPipelineSpec `json:"globalPipeline"`

	// OverwriteConfig allows users to provide raw config sections
	// that will be merged/overwritten into the final configuration
	// Keys are config section names (e.g., "sources.extra", "transforms.custom")
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	OverwriteConfig map[string]runtime.RawExtension `json:"overwriteConfig,omitempty"`

	// ExtraConfigMaps is a list of ConfigMap names to merge into the Vector configuration
	// These ConfigMaps should be in the same namespace
	// TODO: Support merging ConfigMaps in future versions
	// +optional
	ExtraConfigMaps []string `json:"extraConfigMaps,omitempty"`

	// Replicas is the number of Vector instances to run
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Resources defines the compute resources for Vector pods
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Image is the Vector image to use
	// +kubebuilder:default="timberio/vector:latest-debian"
	// +optional
	Image string `json:"image,omitempty"`
}

// VectorSyslogConfigurationStatus defines the observed state of VectorSyslogConfiguration
type VectorSyslogConfigurationStatus struct {
	// ObservedGeneration is the last observed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigHash is the hash of the rendered configuration
	// +optional
	ConfigHash string `json:"configHash,omitempty"`

	// SelectedSources is the list of selected VectorSocketSource names
	// +optional
	SelectedSources []string `json:"selectedSources,omitempty"`

	// ExposedPorts is the list of exposed ports
	// +optional
	ExposedPorts []ExposedPort `json:"exposedPorts,omitempty"`

	// Conditions represent the current state of the VectorSyslogConfiguration
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase is the current phase of the configuration
	// +optional
	Phase string `json:"phase,omitempty"`
}

// ExposedPort represents an exposed port
type ExposedPort struct {
	// Name is the port name
	Name string `json:"name"`
	// Mode is the port mode (tcp or udp)
	Mode string `json:"mode"`
	// Port is the container port
	Port int32 `json:"port"`
	// NodePort is the node port (if using NodePort service)
	NodePort int32 `json:"nodePort,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Sources",type=integer,JSONPath=`.status.selectedSources | length`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VectorSyslogConfiguration is the Schema for the vectorsyslogconfigurations API
// +kubebuilder:resource:scope=Namespaced
// This CR should be a singleton per namespace
type VectorSyslogConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec VectorSyslogConfigurationSpec `json:"spec"`

	// +optional
	Status VectorSyslogConfigurationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VectorSyslogConfigurationList contains a list of VectorSyslogConfiguration
type VectorSyslogConfigurationList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []VectorSyslogConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VectorSyslogConfiguration{}, &VectorSyslogConfigurationList{})
}
