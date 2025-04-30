/*
Copyright 2025.

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

// SysdigTeamGoSpec defines the desired state of SysdigTeamGo
type SysdigTeamGoSpec struct {
	Team TeamSpec `json:"team,omitempty"`
}

// TeamSpec holds the team‐level settings from the CR
type TeamSpec struct {
	Description string     `json:"description,omitempty"`
	Users       []UserSpec `json:"users,omitempty"`
}

// UserSpec represents one entry in spec.team.users
type UserSpec struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// SysdigTeamGoStatus defines the observed state of SysdigTeamGo
type SysdigTeamGoStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=sysdig-team-go

// SysdigTeamGo is the Schema for the sysdig-team-go API
type SysdigTeamGo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SysdigTeamGoSpec   `json:"spec,omitempty"`
	Status SysdigTeamGoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SysdigTeamGoList contains a list of SysdigTeamGo
type SysdigTeamGoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SysdigTeamGo `json:"items"`
}

type Condition struct {
	Type    string                 `json:"type,omitempty"`
	Status  metav1.ConditionStatus `json:"status,omitempty"`
	Reason  string                 `json:"reason,omitempty"`
	Message string                 `json:"message,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SysdigTeamGo{}, &SysdigTeamGoList{})
}
