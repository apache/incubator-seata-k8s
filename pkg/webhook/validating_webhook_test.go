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

package webhook

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	seatav1 "github.com/apache/seata-k8s/api/v1"
)

func TestValidateServiceName(t *testing.T) {
	testCases := []struct {
		name    string
		service string
		valid   bool
	}{
		{
			name:    "valid lowercase name",
			service: "seata-server",
			valid:   true,
		},
		{
			name:    "valid single character",
			service: "s",
			valid:   true,
		},
		{
			name:    "empty service name",
			service: "",
			valid:   false,
		},
		{
			name:    "uppercase characters",
			service: "Seata-Server",
			valid:   false,
		},
		{
			name:    "starts with hyphen",
			service: "-seata-server",
			valid:   false,
		},
		{
			name:    "ends with hyphen",
			service: "seata-server-",
			valid:   false,
		},
		{
			name:    "too long name",
			service: "seata-server-with-very-long-name-that-exceeds-sixty-three-characters-limit",
			valid:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServiceName(tc.service)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateReplicas(t *testing.T) {
	testCases := []struct {
		name     string
		replicas int32
		valid    bool
	}{
		{
			name:     "valid minimum",
			replicas: 1,
			valid:    true,
		},
		{
			name:     "valid middle",
			replicas: 3,
			valid:    true,
		},
		{
			name:     "valid maximum",
			replicas: 1000,
			valid:    true,
		},
		{
			name:     "zero replicas",
			replicas: 0,
			valid:    false,
		},
		{
			name:     "negative replicas",
			replicas: -1,
			valid:    false,
		},
		{
			name:     "exceeds maximum",
			replicas: 1001,
			valid:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReplicas(tc.replicas)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateImage(t *testing.T) {
	testCases := []struct {
		name  string
		image string
		valid bool
	}{
		{
			name:  "valid simple image",
			image: "apache/seata-server:latest",
			valid: true,
		},
		{
			name:  "valid with registry",
			image: "docker.io/apache/seata-server:latest",
			valid: true,
		},
		{
			name:  "valid without tag",
			image: "apache/seata-server",
			valid: true,
		},
		{
			name:  "empty image",
			image: "",
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateImage(tc.image)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidatePorts(t *testing.T) {
	testCases := []struct {
		name  string
		ports seatav1.Ports
		valid bool
	}{
		{
			name: "valid default ports",
			ports: seatav1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
			valid: true,
		},
		{
			name: "invalid service port zero",
			ports: seatav1.Ports{
				ServicePort: 0,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
			valid: false,
		},
		{
			name: "invalid port exceeds max",
			ports: seatav1.Ports{
				ServicePort: 65536,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
			valid: false,
		},
		{
			name: "duplicate ports",
			ports: seatav1.Ports{
				ServicePort: 8091,
				ConsolePort: 8091,
				RaftPort:    9091,
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePorts(tc.ports)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateStorage(t *testing.T) {
	testCases := []struct {
		name        string
		persistence *seatav1.Persistence
		valid       bool
	}{
		{
			name: "valid storage",
			persistence: &seatav1.Persistence{
				PersistentVolumeClaimSpec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceStorage: resource.MustParse("5Gi"),
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "storage too small",
			persistence: &seatav1.Persistence{
				PersistentVolumeClaimSpec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceStorage: resource.MustParse("512Mi"),
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "storage too large",
			persistence: &seatav1.Persistence{
				PersistentVolumeClaimSpec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceStorage: resource.MustParse("2000Gi"),
						},
					},
				},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStorage(tc.persistence)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateVolumeReclaimPolicy(t *testing.T) {
	testCases := []struct {
		name   string
		policy seatav1.VolumeReclaimPolicy
		valid  bool
	}{
		{
			name:   "valid Retain",
			policy: seatav1.VolumeReclaimPolicyRetain,
			valid:  true,
		},
		{
			name:   "valid Delete",
			policy: seatav1.VolumeReclaimPolicyDelete,
			valid:  true,
		},
		{
			name:   "empty policy",
			policy: "",
			valid:  true,
		},
		{
			name:   "invalid policy",
			policy: "Invalid",
			valid:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVolumeReclaimPolicy(tc.policy)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateEnvironmentVariables(t *testing.T) {
	testCases := []struct {
		name  string
		envs  []apiv1.EnvVar
		valid bool
	}{
		{
			name: "valid environment variables",
			envs: []apiv1.EnvVar{
				{Name: "VAR_NAME", Value: "value"},
				{Name: "var_name2", Value: "value"},
				{Name: "_private", Value: "value"},
			},
			valid: true,
		},
		{
			name: "invalid starts with number",
			envs: []apiv1.EnvVar{
				{Name: "1VAR", Value: "value"},
			},
			valid: false,
		},
		{
			name: "invalid contains hyphen",
			envs: []apiv1.EnvVar{
				{Name: "VAR-NAME", Value: "value"},
			},
			valid: false,
		},
		{
			name: "invalid too long name",
			envs: []apiv1.EnvVar{
				{Name: "VAR_" + string(make([]byte, 250)), Value: "value"},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEnvironmentVariables(tc.envs)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateResourceQuantity(t *testing.T) {
	testCases := []struct {
		name         string
		resourceName string
		quantity     resource.Quantity
		valid        bool
	}{
		{
			name:         "valid positive quantity",
			resourceName: "cpu",
			quantity:     resource.MustParse("100m"),
			valid:        true,
		},
		{
			name:         "valid zero quantity",
			resourceName: "memory",
			quantity:     resource.MustParse("0"),
			valid:        true,
		},
		{
			name:         "invalid negative quantity",
			resourceName: "cpu",
			quantity:     resource.MustParse("-100m"),
			valid:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateResourceQuantity(tc.resourceName, tc.quantity)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}

func TestValidateSeataServer(t *testing.T) {
	testCases := []struct {
		name   string
		server *seatav1.SeataServer
		valid  bool
	}{
		{
			name: "valid SeataServer",
			server: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seata-server",
					Namespace: "default",
				},
				Spec: seatav1.SeataServerSpec{
					ContainerSpec: seatav1.ContainerSpec{
						Image: "apache/seata-server:latest",
					},
					Replicas:    3,
					ServiceName: "seata-server-cluster",
					Ports: seatav1.Ports{
						ServicePort: 8091,
						ConsolePort: 7091,
						RaftPort:    9091,
					},
					Persistence: seatav1.Persistence{
						VolumeReclaimPolicy: seatav1.VolumeReclaimPolicyRetain,
						PersistentVolumeClaimSpec: apiv1.PersistentVolumeClaimSpec{
							Resources: apiv1.ResourceRequirements{
								Requests: apiv1.ResourceList{
									apiv1.ResourceStorage: resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid empty service name",
			server: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seata-server",
					Namespace: "default",
				},
				Spec: seatav1.SeataServerSpec{
					ContainerSpec: seatav1.ContainerSpec{
						Image: "apache/seata-server:latest",
					},
					Replicas:    3,
					ServiceName: "",
				},
			},
			valid: false,
		},
		{
			name:   "nil SeataServer",
			server: nil,
			valid:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errors := ValidateSeataServer(tc.server)
			if (len(errors) == 0) != tc.valid {
				t.Errorf("expected valid=%v, got errors=%v", tc.valid, errors)
			}
		})
	}
}

func TestValidateResourceRequirements(t *testing.T) {
	testCases := []struct {
		name         string
		requirements apiv1.ResourceRequirements
		valid        bool
	}{
		{
			name: "valid resource requirements",
			requirements: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("100m"),
					apiv1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("1000m"),
					apiv1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			valid: true,
		},
		{
			name: "CPU limit less than request",
			requirements: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU: resource.MustParse("1000m"),
				},
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU: resource.MustParse("500m"),
				},
			},
			valid: false,
		},
		{
			name: "Memory limit less than request",
			requirements: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Limits: apiv1.ResourceList{
					apiv1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			valid: false,
		},
		{
			name:         "empty resource requirements",
			requirements: apiv1.ResourceRequirements{},
			valid:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateResourceRequirements(tc.requirements)
			if (err == nil) != tc.valid {
				t.Errorf("expected valid=%v, got error=%v", tc.valid, err)
			}
		})
	}
}
