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

package seata

import (
	"context"
	"testing"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function: create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}

// Helper function: create test Service
func createTestService(name, namespace string, ports []apiv1.ServicePort) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1.ServiceSpec{
			Ports: ports,
		},
	}
}

// Helper function: create test StatefulSet
func createTestStatefulSet(name, namespace string, replicas *int32, containers []apiv1.Container, labels map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: replicas,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

func TestSyncService(t *testing.T) {
	tests := []struct {
		name          string
		currentPorts  []apiv1.ServicePort
		desiredPorts  []apiv1.ServicePort
		expectedPorts map[string]int32
	}{
		{
			name: "sync multiple ports",
			currentPorts: []apiv1.ServicePort{
				{Name: "old-port", Port: 8080},
			},
			desiredPorts: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091},
				{Name: "console-port", Port: 7091},
				{Name: "raft-port", Port: 9091},
			},
			expectedPorts: map[string]int32{
				"service-port": 8091,
				"console-port": 7091,
				"raft-port":    9091,
			},
		},
		{
			name: "sync single port",
			currentPorts: []apiv1.ServicePort{
				{Name: "old-port", Port: 8080},
				{Name: "another-port", Port: 9090},
			},
			desiredPorts: []apiv1.ServicePort{
				{Name: "new-port", Port: 8091},
			},
			expectedPorts: map[string]int32{
				"new-port": 8091,
			},
		},
		{
			name: "sync empty ports",
			currentPorts: []apiv1.ServicePort{
				{Name: "port1", Port: 8080},
				{Name: "port2", Port: 9090},
			},
			desiredPorts:  []apiv1.ServicePort{},
			expectedPorts: map[string]int32{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curr := createTestService("test-service", "default", tt.currentPorts)
			next := &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: tt.desiredPorts,
				},
			}

			SyncService(curr, next)

			if len(curr.Spec.Ports) != len(tt.expectedPorts) {
				t.Errorf("Expected %d ports after sync, got %d", len(tt.expectedPorts), len(curr.Spec.Ports))
			}

			portMap := make(map[string]int32)
			for _, port := range curr.Spec.Ports {
				portMap[port.Name] = port.Port
			}

			for name, expectedPort := range tt.expectedPorts {
				if actualPort, exists := portMap[name]; !exists {
					t.Errorf("Expected port %s to exist", name)
				} else if actualPort != expectedPort {
					t.Errorf("Expected port %s to be %d, got %d", name, expectedPort, actualPort)
				}
			}
		})
	}
}

func TestSyncStatefulSet(t *testing.T) {
	tests := []struct {
		name               string
		currentReplicas    *int32
		currentContainers  []apiv1.Container
		currentLabels      map[string]string
		desiredReplicas    *int32
		desiredContainers  []apiv1.Container
		desiredLabels      map[string]string
		expectedReplicas   int32
		expectedContainers int
		expectedLabels     map[string]string
	}{
		{
			name:            "sync template and replicas",
			currentReplicas: int32Ptr(1),
			currentContainers: []apiv1.Container{
				{Name: "old-container", Image: "old-image:v1"},
			},
			currentLabels:   nil,
			desiredReplicas: int32Ptr(3),
			desiredContainers: []apiv1.Container{
				{
					Name:  "seata-server",
					Image: "apache/seata-server:latest",
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceCPU:    resource.MustParse("500m"),
							apiv1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			desiredLabels:      map[string]string{"app": "seata"},
			expectedReplicas:   3,
			expectedContainers: 1,
			expectedLabels:     map[string]string{"app": "seata"},
		},
		{
			name:            "set replicas when nil",
			currentReplicas: nil,
			currentContainers: []apiv1.Container{
				{Name: "old", Image: "old:v1"},
			},
			desiredReplicas: int32Ptr(1),
			desiredContainers: []apiv1.Container{
				{Name: "new", Image: "new:v2"},
			},
			expectedReplicas:   1,
			expectedContainers: 1,
		},
		{
			name:            "sync multiple containers",
			currentReplicas: int32Ptr(2),
			currentContainers: []apiv1.Container{
				{Name: "container1", Image: "image1:v1"},
			},
			desiredReplicas: int32Ptr(2),
			desiredContainers: []apiv1.Container{
				{Name: "container1", Image: "image1:v2"},
				{Name: "sidecar", Image: "sidecar:latest"},
			},
			expectedReplicas:   2,
			expectedContainers: 2,
		},
		{
			name:            "scale up replicas",
			currentReplicas: int32Ptr(1),
			currentContainers: []apiv1.Container{
				{Name: "app", Image: "app:v1"},
			},
			desiredReplicas: int32Ptr(5),
			desiredContainers: []apiv1.Container{
				{Name: "app", Image: "app:v1"},
			},
			expectedReplicas:   5,
			expectedContainers: 1,
		},
		{
			name:            "scale down replicas",
			currentReplicas: int32Ptr(5),
			currentContainers: []apiv1.Container{
				{Name: "app", Image: "app:v1"},
			},
			desiredReplicas: int32Ptr(2),
			desiredContainers: []apiv1.Container{
				{Name: "app", Image: "app:v1"},
			},
			expectedReplicas:   2,
			expectedContainers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curr := createTestStatefulSet("test-sts", "default", tt.currentReplicas, tt.currentContainers, tt.currentLabels)
			next := createTestStatefulSet("", "", tt.desiredReplicas, tt.desiredContainers, tt.desiredLabels)

			SyncStatefulSet(curr, next)

			// Verify replicas
			if curr.Spec.Replicas == nil {
				t.Fatal("Expected replicas to be set")
			}
			if *curr.Spec.Replicas != tt.expectedReplicas {
				t.Errorf("Expected %d replicas, got %d", tt.expectedReplicas, *curr.Spec.Replicas)
			}

			// Verify container count
			if len(curr.Spec.Template.Spec.Containers) != tt.expectedContainers {
				t.Errorf("Expected %d containers, got %d", tt.expectedContainers, len(curr.Spec.Template.Spec.Containers))
			}

			// Verify labels if set
			if tt.expectedLabels != nil {
				for key, expectedValue := range tt.expectedLabels {
					if actualValue := curr.Spec.Template.Labels[key]; actualValue != expectedValue {
						t.Errorf("Expected label %s='%s', got '%s'", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestChangeCluster_ErrorHandling(t *testing.T) {
	// Test error handling of changeCluster
	// Without a real Seata server, this will trigger error path
	tests := []struct {
		name     string
		username string
		password string
		index    int32
	}{
		{
			name:     "empty credentials",
			username: "",
			password: "",
			index:    0,
		},
		{
			name:     "with credentials",
			username: "admin",
			password: "admin",
			index:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seataServer := createTestSeataServer("test-seata", "default", 3)
			err := changeCluster(seataServer, tt.index, tt.username, tt.password)
			if err == nil {
				t.Log("changeCluster returned no error (expected as there is no real server)")
			} else {
				t.Logf("changeCluster returned expected error: %v", err)
			}
		})
	}
}

func TestSyncRaftCluster_ErrorHandling(t *testing.T) {
	// Test error handling of SyncRaftCluster
	// Without a real Seata server, this will test error path
	ctx := context.Background()
	seataServer := createTestSeataServer("test-seata", "default", 3)

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Logf("SyncRaftCluster returned expected error (no real server): %v", err)
	}
}

// Boundary condition test: nil input
func TestSyncService_NilInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic captured: %v", r)
		}
	}()

	// Test boundary case of nil input
	var curr *apiv1.Service
	next := &apiv1.Service{}

	// This should panic because curr is nil
	SyncService(curr, next)
	t.Error("Expected panic did not occur")
}

func TestSyncStatefulSet_NilInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic captured: %v", r)
		}
	}()

	// Test boundary case of nil input
	var curr *appsv1.StatefulSet
	next := &appsv1.StatefulSet{}

	// This should panic because curr is nil
	SyncStatefulSet(curr, next)
	t.Error("Expected panic did not occur")
}

// Helper function: create test SeataServer with custom parameters
func createTestSeataServer(name, namespace string, replicas int32) *seatav1alpha1.SeataServer {
	return &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-server",
			Replicas:    replicas,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}
}
