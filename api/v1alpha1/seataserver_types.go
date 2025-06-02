/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultSeataSessionStorageVolumeSize = "5Gi"
	DefaultSeataServerImage              = "seataio/seata-server:latest"
)

// SeataServerSpec defines the desired state of SeataServer
type SeataServerSpec struct {
	ContainerSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=seata-server-cluster
	ServiceName string `json:"serviceName"`

	// +kubebuilder:validation:Optional
	Ports Ports `json:"ports,omitempty"`

	// +kubebuilder:validation:Optional
	Persistence Persistence `json:"persistence,omitempty"`

	// +kubebuilder:validation:Optional
	Store Store `json:"store,omitempty"`
}

func (s *SeataServerSpec) withDefaults() (changed bool) {
	if s.ContainerName == "" {
		s.ContainerName = "seata-server"
		changed = true
	}
	if s.ServiceName == "" {
		s.ServiceName = "seata-server-cluster"
		changed = true
	}
	if s.Image == "" {
		s.Image = DefaultSeataServerImage
		changed = true
	}
	changed = s.Ports.withDefaults() || changed
	changed = s.Persistence.withDefaults() || changed

	if s.Store.Resources.Requests.Storage == nil {
		defaultStorage := resource.MustParse("5Gi")
		s.Store.Resources.Requests.Storage = &defaultStorage
		changed = true
	}

	return changed
}

type ServerErrorType string

const (
	ErrorTypeK8s_SeataServer     ServerErrorType = "k8s-seata-server"
	ErrorTypeK8s_HeadlessService ServerErrorType = "k8s-headless-service"
	ErrorTypeK8s_Pvc             ServerErrorType = "k8s-pvc"
	ErrorTypeK8s_StatefulSet     ServerErrorType = "k8s-statefulset"
	ErrorTypeRuntime             ServerErrorType = "runtime"
)

func (e ServerErrorType) String() string {
	return string(e)
}

// SeataServerError defines the error of SeataServer
type SeataServerError struct {
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Timestamp metav1.Time `json:"timestamp"`
}

// SeataServerStatus defines the observed state of SeataServer
type SeataServerStatus struct {
	Synchronized  bool               `json:"synchronized"`
	Replicas      int32              `json:"replicas"`
	ReadyReplicas int32              `json:"readyReplicas,omitempty"`
	Errors        []SeataServerError `json:"errors,omitempty"`
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

func (s *SeataServer) WithDefaults() bool {
	return s.Spec.withDefaults()
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
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="seataio/seata-server:latest"
	Image string `json:"image"`
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

func (p *Ports) withDefaults() bool {
	if *p == (Ports{}) {
		p.ConsolePort = 7091
		p.ServicePort = 8091
		p.RaftPort = 9091
		return true
	}
	return false
}

// Store defines the storage configuration for SeataServer
// +kubebuilder:object:generate=true
type Store struct {
	// +kubebuilder:validation:Optional
	Resources StorageResources `json:"resources,omitempty"`
}

type StorageResources struct {
	// +kubebuilder:validation:Optional
	Requests ResourceList `json:"requests,omitempty"`
}

type ResourceList struct {
	// +kubebuilder:validation:Optional
	Storage *resource.Quantity `json:"storage,omitempty"`
}

type Persistence struct {
	// VolumeReclaimPolicy is a seata operator configuration. If it's set to Delete,
	// the corresponding PVCs will be deleted by the operator when seata server cluster is deleted.
	// The default value is Retain.
	// +kubebuilder:validation:Enum="Delete";"Retain"
	VolumeReclaimPolicy VolumeReclaimPolicy `json:"volumeReclaimPolicy,omitempty"`
	// PersistentVolumeClaimSpec is the spec to describe PVC for the container
	// This field is optional. If no PVC is specified default persistent volume
	// will get created.
	PersistentVolumeClaimSpec apiv1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type VolumeReclaimPolicy string

const (
	VolumeReclaimPolicyRetain VolumeReclaimPolicy = "Retain"
	VolumeReclaimPolicyDelete VolumeReclaimPolicy = "Delete"
)

func (p *Persistence) withDefaults() (changed bool) {
	if p.VolumeReclaimPolicy != VolumeReclaimPolicyDelete && p.VolumeReclaimPolicy != VolumeReclaimPolicyRetain {
		changed = true
		p.VolumeReclaimPolicy = VolumeReclaimPolicyRetain
	}
	p.PersistentVolumeClaimSpec.AccessModes = []apiv1.PersistentVolumeAccessMode{
		apiv1.ReadWriteOnce,
	}

	storage, _ := p.PersistentVolumeClaimSpec.Resources.Requests["storage"]
	if storage.IsZero() {
		p.PersistentVolumeClaimSpec.Resources.Requests = apiv1.ResourceList{
			apiv1.ResourceStorage: resource.MustParse(DefaultSeataSessionStorageVolumeSize),
		}
		changed = true
	}
	return changed
}

func init() {
	SchemeBuilder.Register(&SeataServer{}, &SeataServerList{})
}
