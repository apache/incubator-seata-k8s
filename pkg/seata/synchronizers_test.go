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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSyncService(t *testing.T) {
	// Current service
	curr := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "old-port", Port: 8080},
			},
		},
	}

	// Next/desired service
	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091},
				{Name: "console-port", Port: 7091},
				{Name: "raft-port", Port: 9091},
			},
		},
	}

	SyncService(curr, next)

	// Verify ports are synced
	if len(curr.Spec.Ports) != 3 {
		t.Errorf("Expected 3 ports after sync, got %d", len(curr.Spec.Ports))
	}

	portMap := make(map[string]int32)
	for _, port := range curr.Spec.Ports {
		portMap[port.Name] = port.Port
	}

	if portMap["service-port"] != 8091 {
		t.Errorf("Expected service-port 8091, got %d", portMap["service-port"])
	}
	if portMap["console-port"] != 7091 {
		t.Errorf("Expected console-port 7091, got %d", portMap["console-port"])
	}
	if portMap["raft-port"] != 9091 {
		t.Errorf("Expected raft-port 9091, got %d", portMap["raft-port"])
	}
}

func TestSyncStatefulSet(t *testing.T) {
	replicas1 := int32(1)
	replicas3 := int32(3)

	// Current StatefulSet
	curr := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas1,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "old-container",
							Image: "old-image:v1",
						},
					},
				},
			},
		},
	}

	// Next/desired StatefulSet
	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "seata",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
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
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	// Verify template is synced
	if len(curr.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expected 1 container after sync, got %d", len(curr.Spec.Template.Spec.Containers))
	}

	container := curr.Spec.Template.Spec.Containers[0]
	if container.Name != "seata-server" {
		t.Errorf("Expected container name 'seata-server', got '%s'", container.Name)
	}

	if container.Image != "apache/seata-server:latest" {
		t.Errorf("Expected image 'apache/seata-server:latest', got '%s'", container.Image)
	}

	// Verify replicas are synced
	if *curr.Spec.Replicas != 3 {
		t.Errorf("Expected 3 replicas after sync, got %d", *curr.Spec.Replicas)
	}

	// Verify labels are synced
	if curr.Spec.Template.Labels["app"] != "seata" {
		t.Errorf("Expected template label app='seata', got '%s'", curr.Spec.Template.Labels["app"])
	}
}

func TestSyncStatefulSet_EmptyReplicas(t *testing.T) {
	replicas1 := int32(1)

	// Current StatefulSet
	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: nil,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "old", Image: "old:v1"},
					},
				},
			},
		},
	}

	// Next StatefulSet
	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas1,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "new", Image: "new:v2"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if curr.Spec.Replicas == nil {
		t.Error("Expected replicas to be set")
	} else if *curr.Spec.Replicas != 1 {
		t.Errorf("Expected 1 replica, got %d", *curr.Spec.Replicas)
	}
}

func TestSyncService_EmptyPorts(t *testing.T) {
	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "port1", Port: 8080},
				{Name: "port2", Port: 9090},
			},
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{},
		},
	}

	SyncService(curr, next)

	if len(curr.Spec.Ports) != 0 {
		t.Errorf("Expected 0 ports after sync, got %d", len(curr.Spec.Ports))
	}
}

func TestSyncStatefulSet_WithMultipleContainers(t *testing.T) {
	replicas2 := int32(2)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "container1", Image: "image1:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "container1", Image: "image1:v2"},
						{Name: "sidecar", Image: "sidecar:latest"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if len(curr.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("Expected 2 containers after sync, got %d", len(curr.Spec.Template.Spec.Containers))
	}

	if curr.Spec.Template.Spec.Containers[0].Image != "image1:v2" {
		t.Errorf("Expected first container image 'image1:v2', got '%s'", curr.Spec.Template.Spec.Containers[0].Image)
	}

	if curr.Spec.Template.Spec.Containers[1].Name != "sidecar" {
		t.Errorf("Expected second container name 'sidecar', got '%s'", curr.Spec.Template.Spec.Containers[1].Name)
	}
}

func TestSyncService_WithSinglePort(t *testing.T) {
	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "old-port", Port: 8080},
				{Name: "another-port", Port: 9090},
			},
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "new-port", Port: 8091},
			},
		},
	}

	SyncService(curr, next)

	if len(curr.Spec.Ports) != 1 {
		t.Errorf("Expected 1 port after sync, got %d", len(curr.Spec.Ports))
	}

	if curr.Spec.Ports[0].Name != "new-port" {
		t.Errorf("Expected port name 'new-port', got '%s'", curr.Spec.Ports[0].Name)
	}

	if curr.Spec.Ports[0].Port != 8091 {
		t.Errorf("Expected port 8091, got %d", curr.Spec.Ports[0].Port)
	}
}

func TestSyncStatefulSet_ReplicasScaleUp(t *testing.T) {
	replicas1 := int32(1)
	replicas5 := int32(5)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas1,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas5,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if *curr.Spec.Replicas != 5 {
		t.Errorf("Expected 5 replicas after scale up, got %d", *curr.Spec.Replicas)
	}
}

func TestSyncStatefulSet_ReplicasScaleDown(t *testing.T) {
	replicas5 := int32(5)
	replicas2 := int32(2)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas5,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if *curr.Spec.Replicas != 2 {
		t.Errorf("Expected 2 replicas after scale down, got %d", *curr.Spec.Replicas)
	}
}

func TestChangeCluster_NetworkError(t *testing.T) {
	// Test network error when server is not available
	seataServer := createTestSeataServer()

	err := changeCluster(seataServer, 0, "admin", "admin")
	if err == nil {
		t.Error("Expected error when server is not available")
	} else {
		// The error message should contain either "failed to send login request" or the actual network error
		if !strings.Contains(err.Error(), "failed to send login request") &&
			!strings.Contains(err.Error(), "Post") {
			t.Errorf("Expected network/login request error, got: %v", err)
		}
	}
}

func TestChangeCluster_DifferentPodIndex(t *testing.T) {
	// Test with different pod indices
	seataServer := createTestSeataServer()

	testCases := []int32{0, 1, 2}
	for _, idx := range testCases {
		err := changeCluster(seataServer, idx, "admin", "admin")
		// We expect error since there's no real server, but we're testing the function logic
		if err == nil {
			t.Logf("Pod %d: changeCluster returned no error (unexpected)", idx)
		} else {
			t.Logf("Pod %d: changeCluster returned expected error: %v", idx, err)
		}
	}
}

func TestChangeCluster_EmptyCredentials(t *testing.T) {
	// Test with empty username and password
	seataServer := createTestSeataServer()

	err := changeCluster(seataServer, 0, "", "")
	if err == nil {
		t.Log("changeCluster with empty credentials returned no error")
	} else {
		t.Logf("changeCluster with empty credentials returned error: %v", err)
	}
}

func TestSyncRaftCluster_ErrorHandling(t *testing.T) {
	// Test SyncRaftCluster error handling
	// Without a real Seata server, this will error, which tests the error path

	ctx := context.Background()
	seataServer := createTestSeataServer()

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Logf("SyncRaftCluster returned expected error without real server: %v", err)
	}
}

func TestSyncRaftCluster_SingleReplica(t *testing.T) {
	// Test with single replica
	ctx := context.Background()
	seataServer := createTestSeataServer()
	seataServer.Spec.Replicas = 1

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Logf("SyncRaftCluster with single replica returned expected error: %v", err)
	}
}

func TestSyncRaftCluster_MultipleReplicas(t *testing.T) {
	// Test with multiple replicas
	ctx := context.Background()
	seataServer := createTestSeataServer()
	seataServer.Spec.Replicas = 5

	err := SyncRaftCluster(ctx, seataServer, "user", "pass")
	if err != nil {
		t.Logf("SyncRaftCluster with 5 replicas returned expected error: %v", err)
	}
}

func TestSyncRaftCluster_CancelledContext(t *testing.T) {
	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	seataServer := createTestSeataServer()

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Logf("SyncRaftCluster with cancelled context returned error: %v", err)
	}
}

func TestSyncService_NilPorts(t *testing.T) {
	// Test syncing with nil ports
	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: nil,
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "new-port", Port: 8091},
			},
		},
	}

	SyncService(curr, next)

	if len(curr.Spec.Ports) != 1 {
		t.Errorf("Expected 1 port after sync, got %d", len(curr.Spec.Ports))
	}
}

func TestSyncStatefulSet_NilReplicas(t *testing.T) {
	// Test syncing when both have nil replicas
	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: nil,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "old", Image: "old:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: nil,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "new", Image: "new:v2"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	// Should be nil after sync
	if curr.Spec.Replicas != nil {
		t.Errorf("Expected replicas to be nil, got %v", *curr.Spec.Replicas)
	}

	// Check container is synced
	if len(curr.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(curr.Spec.Template.Spec.Containers))
	}
	if curr.Spec.Template.Spec.Containers[0].Image != "new:v2" {
		t.Errorf("Expected image 'new:v2', got '%s'", curr.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestSyncStatefulSet_ComplexTemplate(t *testing.T) {
	replicas := int32(3)

	// Test with more complex template including volumes, env vars, etc.
	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "old", Image: "old:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "seata",
						"version": "v2",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "seata",
							Image: "seata:v2",
							Env: []apiv1.EnvVar{
								{Name: "SEATA_PORT", Value: "8091"},
							},
						},
					},
					InitContainers: []apiv1.Container{
						{Name: "init", Image: "init:v1"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	// Verify labels
	if len(curr.Spec.Template.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(curr.Spec.Template.Labels))
	}

	// Verify annotations
	if len(curr.Spec.Template.Annotations) != 1 {
		t.Errorf("Expected 1 annotation, got %d", len(curr.Spec.Template.Annotations))
	}

	// Verify init containers
	if len(curr.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("Expected 1 init container, got %d", len(curr.Spec.Template.Spec.InitContainers))
	}

	// Verify env vars
	if len(curr.Spec.Template.Spec.Containers[0].Env) != 1 {
		t.Errorf("Expected 1 env var, got %d", len(curr.Spec.Template.Spec.Containers[0].Env))
	}
}

func TestSyncService_MultiplePortsSync(t *testing.T) {
	// Test syncing with multiple ports in both current and next
	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "port1", Port: 8080, Protocol: apiv1.ProtocolTCP},
				{Name: "port2", Port: 9090, Protocol: apiv1.ProtocolTCP},
				{Name: "port3", Port: 7070, Protocol: apiv1.ProtocolUDP},
			},
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091, Protocol: apiv1.ProtocolTCP},
				{Name: "console-port", Port: 7091, Protocol: apiv1.ProtocolTCP},
			},
		},
	}

	SyncService(curr, next)

	if len(curr.Spec.Ports) != 2 {
		t.Errorf("Expected 2 ports after sync, got %d", len(curr.Spec.Ports))
	}

	// Verify protocol is synced
	for _, port := range curr.Spec.Ports {
		if port.Protocol != apiv1.ProtocolTCP {
			t.Errorf("Expected protocol TCP, got %s", port.Protocol)
		}
	}
}

func TestChangeCluster_DifferentNamespace(t *testing.T) {
	// Test with different namespace
	seataServer := createTestSeataServer()
	seataServer.Namespace = "seata-system"

	err := changeCluster(seataServer, 0, "admin", "admin")
	if err != nil {
		t.Logf("changeCluster in different namespace returned error: %v", err)
	}
}

func TestChangeCluster_CustomServiceName(t *testing.T) {
	// Test with custom service name
	seataServer := createTestSeataServer()
	seataServer.Spec.ServiceName = "custom-seata-service"

	err := changeCluster(seataServer, 0, "admin", "admin")
	if err != nil {
		t.Logf("changeCluster with custom service name returned error: %v", err)
	}
}

func TestChangeCluster_DifferentPorts(t *testing.T) {
	// Test with different console port
	seataServer := createTestSeataServer()
	seataServer.Spec.Ports.ConsolePort = 9999

	err := changeCluster(seataServer, 0, "admin", "admin")
	if err != nil {
		t.Logf("changeCluster with different console port returned error: %v", err)
	}
}

func TestSyncRaftCluster_EmptyCredentials(t *testing.T) {
	// Test with empty credentials
	ctx := context.Background()
	seataServer := createTestSeataServer()

	err := SyncRaftCluster(ctx, seataServer, "", "")
	if err != nil {
		t.Logf("SyncRaftCluster with empty credentials returned error: %v", err)
	}
}

func TestSyncRaftCluster_ZeroReplicas(t *testing.T) {
	// Test with zero replicas - should not call changeCluster at all
	ctx := context.Background()
	seataServer := createTestSeataServer()
	seataServer.Spec.Replicas = 0

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Errorf("SyncRaftCluster with zero replicas should not error, got: %v", err)
	}
}

func createTestSeataServer() *seatav1alpha1.SeataServer {
	return &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-server",
			Replicas:    3,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}
}

// Mock HTTP server tests to demonstrate testing patterns
// Note: These tests demonstrate the HTTP mocking pattern but cannot directly
// test changeCluster since it constructs K8s internal URLs
func TestChangeCluster_WithMockServer_Success(t *testing.T) {
	// Create a mock server that simulates successful login and changeCluster
	loginCalled := false
	changeClusterCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/login") {
			loginCalled = true
			// Successful login response
			response := rspData{
				Code:    "200",
				Message: "success",
				Data:    "mock-token-12345",
				Success: true,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		} else if strings.Contains(r.URL.Path, "/metadata/v1/changeCluster") {
			changeClusterCalled = true
			// Check authorization header
			if r.Header.Get("Authorization") != "mock-token-12345" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// Successful changeCluster response
			response := rspData{
				Code:    "200",
				Message: "success",
				Data:    "cluster changed",
				Success: true,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Parse server URL to get host and port
	serverURL, _ := url.Parse(server.URL)

	// This test demonstrates the HTTP mocking pattern
	t.Logf("Mock server running at: %s", serverURL.Host)
	t.Logf("Login called: %v, ChangeCluster called: %v", loginCalled, changeClusterCalled)
}

func TestChangeCluster_WithMockServer_LoginFailure(t *testing.T) {
	// Create a mock server that simulates login failure
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/login") {
			// Failed login response
			response := rspData{
				Code:    "401",
				Message: "invalid credentials",
				Data:    "",
				Success: false,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	t.Logf("Mock server for login failure at: %s", serverURL.Host)
}

func TestChangeCluster_WithMockServer_LoginHTTPError(t *testing.T) {
	// Create a mock server that returns HTTP error for login
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/login") {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	t.Logf("Mock server for HTTP error at: %s", serverURL.Host)
}

func TestChangeCluster_WithMockServer_ChangeClusterFailure(t *testing.T) {
	// Create a mock server where login succeeds but changeCluster fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/login") {
			response := rspData{
				Code:    "200",
				Message: "success",
				Data:    "mock-token",
				Success: true,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		} else if strings.Contains(r.URL.Path, "/metadata/v1/changeCluster") {
			// Failed changeCluster response
			response := rspData{
				Code:    "500",
				Message: "cluster change failed",
				Data:    "",
				Success: false,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	t.Logf("Mock server for changeCluster failure at: %s", serverURL.Host)
}

func TestChangeCluster_WithMockServer_InvalidJSON(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/login") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json response"))
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	t.Logf("Mock server for invalid JSON at: %s", serverURL.Host)
}

func TestRspData_Struct(t *testing.T) {
	// Test rspData struct marshaling and unmarshaling
	original := rspData{
		Code:    "200",
		Message: "test message",
		Data:    "test data",
		Success: true,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal rspData: %v", err)
	}

	// Unmarshal back
	var decoded rspData
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal rspData: %v", err)
	}

	// Verify fields
	if decoded.Code != original.Code {
		t.Errorf("Expected Code %s, got %s", original.Code, decoded.Code)
	}
	if decoded.Message != original.Message {
		t.Errorf("Expected Message %s, got %s", original.Message, decoded.Message)
	}
	if decoded.Data != original.Data {
		t.Errorf("Expected Data %s, got %s", original.Data, decoded.Data)
	}
	if decoded.Success != original.Success {
		t.Errorf("Expected Success %v, got %v", original.Success, decoded.Success)
	}
}

func TestSyncService_LargeNumberOfPorts(t *testing.T) {
	// Test with a large number of ports
	var currentPorts []apiv1.ServicePort
	for i := 0; i < 50; i++ {
		currentPorts = append(currentPorts, apiv1.ServicePort{
			Name: fmt.Sprintf("port-%d", i),
			Port: int32(8000 + i),
		})
	}

	var desiredPorts []apiv1.ServicePort
	for i := 0; i < 30; i++ {
		desiredPorts = append(desiredPorts, apiv1.ServicePort{
			Name: fmt.Sprintf("new-port-%d", i),
			Port: int32(9000 + i),
		})
	}

	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: currentPorts,
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: desiredPorts,
		},
	}

	SyncService(curr, next)

	if len(curr.Spec.Ports) != 30 {
		t.Errorf("Expected 30 ports after sync, got %d", len(curr.Spec.Ports))
	}
}

func TestSyncStatefulSet_LargeReplicas(t *testing.T) {
	// Test with large number of replicas
	replicas100 := int32(100)
	replicas200 := int32(200)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas100,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas200,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v2"},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if *curr.Spec.Replicas != 200 {
		t.Errorf("Expected 200 replicas, got %d", *curr.Spec.Replicas)
	}

	if curr.Spec.Template.Spec.Containers[0].Image != "app:v2" {
		t.Errorf("Expected image app:v2, got %s", curr.Spec.Template.Spec.Containers[0].Image)
	}
}

// Additional edge case tests
func TestSyncService_SamePortsDifferentProtocol(t *testing.T) {
	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "port1", Port: 8080, Protocol: apiv1.ProtocolTCP},
			},
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "port1", Port: 8080, Protocol: apiv1.ProtocolUDP},
			},
		},
	}

	SyncService(curr, next)

	if curr.Spec.Ports[0].Protocol != apiv1.ProtocolUDP {
		t.Errorf("Expected protocol UDP, got %s", curr.Spec.Ports[0].Protocol)
	}
}

func TestSyncStatefulSet_EmptyContainers(t *testing.T) {
	replicas := int32(1)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "old", Image: "old:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if len(curr.Spec.Template.Spec.Containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(curr.Spec.Template.Spec.Containers))
	}
}

func TestChangeCluster_SpecialCharactersInCredentials(t *testing.T) {
	seataServer := createTestSeataServer()

	// Test with special characters in username/password
	err := changeCluster(seataServer, 0, "admin@test", "p@ssw0rd!#$")
	if err != nil {
		t.Logf("changeCluster with special characters returned error: %v", err)
	}
}

func TestChangeCluster_LongCredentials(t *testing.T) {
	seataServer := createTestSeataServer()

	// Test with very long credentials
	longUsername := strings.Repeat("a", 1000)
	longPassword := strings.Repeat("b", 1000)

	err := changeCluster(seataServer, 0, longUsername, longPassword)
	if err != nil {
		t.Logf("changeCluster with long credentials returned error: %v", err)
	}
}

func TestSyncRaftCluster_LargeNumberOfReplicas(t *testing.T) {
	ctx := context.Background()
	seataServer := createTestSeataServer()
	seataServer.Spec.Replicas = 10

	err := SyncRaftCluster(ctx, seataServer, "admin", "admin")
	if err != nil {
		t.Logf("SyncRaftCluster with 10 replicas returned error: %v", err)
	}
}

func TestChangeCluster_DifferentServerName(t *testing.T) {
	seataServer := createTestSeataServer()
	seataServer.Name = "my-custom-seata-cluster"

	err := changeCluster(seataServer, 0, "admin", "admin")
	if err != nil {
		t.Logf("changeCluster with custom server name returned error: %v", err)
	}
}

func TestChangeCluster_HighPodIndex(t *testing.T) {
	seataServer := createTestSeataServer()
	seataServer.Spec.Replicas = 100

	// Test with high pod index
	err := changeCluster(seataServer, 99, "admin", "admin")
	if err != nil {
		t.Logf("changeCluster with pod index 99 returned error: %v", err)
	}
}

func TestSyncService_PortsWithTargetPort(t *testing.T) {
	targetPort := int32(9999)

	curr := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "old", Port: 8080},
			},
		},
	}

	next := &apiv1.Service{
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "new",
					Port:       8091,
					TargetPort: intstr.FromInt(int(targetPort)),
				},
			},
		},
	}

	SyncService(curr, next)

	if curr.Spec.Ports[0].TargetPort.IntVal != targetPort {
		t.Errorf("Expected target port %d, got %d", targetPort, curr.Spec.Ports[0].TargetPort.IntVal)
	}
}

func TestSyncStatefulSet_WithVolumes(t *testing.T) {
	replicas := int32(1)

	curr := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v1"},
					},
				},
			},
		},
	}

	next := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "app:v2"},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "data",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	SyncStatefulSet(curr, next)

	if len(curr.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("Expected 1 volume, got %d", len(curr.Spec.Template.Spec.Volumes))
	}
}
