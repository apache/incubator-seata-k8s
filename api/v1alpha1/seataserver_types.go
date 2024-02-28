/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SeataServerSpec defines the desired state of SeataServer
type SeataServerSpec struct {
	ContainerSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=seata-server
	ServiceName string `json:"serviceName"`

	// +kubebuilder:validation:Optional
	Ports Ports `json:"ports,omitempty"`

	Store Store `json:"store"`
}

// SeataServerStatus defines the observed state of SeataServer
type SeataServerStatus struct {
	Synchronized  bool  `json:"synchronized"`
	Replicas      int32 `json:"replicas"`
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SeataServer is the Schema for the seataservers API
type SeataServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SeataServerSpec   `json:"spec,omitempty"`
	Status SeataServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SeataServerList contains a list of SeataServer
type SeataServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SeataServer `json:"items"`
}

type ContainerSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=seata-server
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	// +kubebuilder:validation:Optional
	Env []apiv1.EnvVar `json:"env"`
	// +kubebuilder:validation:Optional
	Resources apiv1.ResourceRequirements `json:"resources"`
}

type Ports struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=7091
	ConsolePort int32 `json:"consolePort"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=8091
	ServicePort int32 `json:"servicePort"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=9091
	RaftPort int32 `json:"raftPort"`
}

type Store struct {
	Resources apiv1.ResourceRequirements `json:"resources"`
}

func init() {
	SchemeBuilder.Register(&SeataServer{}, &SeataServerList{})
}
