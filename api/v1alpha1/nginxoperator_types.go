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

const (
	ConditionTypeReconcileFailed = "ReconcileFailed"

	ReasonCustomResourceInvalid  = "OperandInvalid"
	ReasonDeploymentNotAvailable = "OperandDeploymentNotAvailable"
	ReasonCreateDeploymentFailed = "OperandCreateDeploymentFailed"
	ReasonUpdateDeploymentFailed = "OperandUpdateDeploymentFailed"
	ReasonServiceNotAvailable    = "OperandServiceNotAvailable"
	ReasonCreateServiceFailed    = "OperandCreateServiceFailed"
	ReasonUpdateServiceFailed    = "OperandUpdateServiceFailed"
	ReasonIngressNotAvailable    = "OperandIngressNotAvailable"
	ReasonCreateIngressFailed    = "OperandCreateIngressFailed"
	ReasonUpdateIngressFailed    = "OperandUpdateIngressFailed"
	ReasonSucceeded              = "OperatorSucceeded"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NginxOperatorSpec defines the desired state of NginxOperator
type NginxOperatorSpec struct {

	// Host is the url on which the nginx web server will be exposed
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`
	Hostname *string `json:"hostname"`

	// Port is the number of port on which the nginx web server will be exposed
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65635
	// +kubebuilder:default=80
	Port *int32 `json:"port,omitempty"`

	// Image define the image name of the nginx based container
	// +kubebuilder:validation:Pattern=`[a-z0-9]+([._-]{1,2}[a-z0-9]+)*`
	// +kubebuilder:validation:Required
	Image *string `json:"image"`

	// Issuer defines the name and the namespace of certificate issuer in the kubernetes cluster
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9])([a-zA-Z0-9\-\.])*([a-zA-Z0-9])?\/([a-zA-Z0-9])([a-zA-Z0-9\-\.])*([a-zA-Z0-9])?$`
	// +kubebuilder:validation:Required
	Issuer *string `json:"issuer"`

	// Replicas is the number of deployed pod of Nginx
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas *int32 `json:"replicas,omitempty"`
}

// NginxOperatorStatus defines the observed state of NginxOperator
type NginxOperatorStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NginxOperator is the Schema for the nginxoperators API
type NginxOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxOperatorSpec   `json:"spec,omitempty"`
	Status NginxOperatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NginxOperatorList contains a list of NginxOperator
type NginxOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxOperator{}, &NginxOperatorList{})
}
