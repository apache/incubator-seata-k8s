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
	"strings"
	"testing"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeLabels(t *testing.T) {
	name := "test-seata"
	labels := makeLabels(name)

	if labels["cr_name"] != name {
		t.Errorf("Expected label cr_name='%s', got '%s'", name, labels["cr_name"])
	}
}

func TestMakeHeadlessService(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-cluster",
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	svc := MakeHeadlessService(seataServer)

	if svc.Name != "seata-cluster" {
		t.Errorf("Expected service name 'seata-cluster', got '%s'", svc.Name)
	}

	if svc.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", svc.Namespace)
	}

	if svc.Spec.ClusterIP != "None" {
		t.Errorf("Expected ClusterIP 'None', got '%s'", svc.Spec.ClusterIP)
	}

	if len(svc.Spec.Ports) != 3 {
		t.Errorf("Expected 3 ports, got %d", len(svc.Spec.Ports))
	}

	// Verify ports
	portMap := make(map[string]int32)
	for _, port := range svc.Spec.Ports {
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

func TestMakeStatefulSet(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
			Labels: map[string]string{
				"app": "seata",
			},
			Annotations: map[string]string{
				"description": "test seata server",
			},
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("500m"),
						apiv1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
			ServiceName: "seata-cluster",
			Replicas:    3,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
			Persistence: seatav1alpha1.Persistence{
				PersistentVolumeClaimSpec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceStorage: resource.MustParse("5Gi"),
						},
					},
				},
			},
		},
	}

	sts := MakeStatefulSet(seataServer)

	if sts.Name != "test-seata" {
		t.Errorf("Expected StatefulSet name 'test-seata', got '%s'", sts.Name)
	}

	if sts.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", sts.Namespace)
	}

	if *sts.Spec.Replicas != 3 {
		t.Errorf("Expected 3 replicas, got %d", *sts.Spec.Replicas)
	}

	if sts.Spec.ServiceName != "seata-cluster" {
		t.Errorf("Expected service name 'seata-cluster', got '%s'", sts.Spec.ServiceName)
	}

	// Check VolumeClaimTemplates
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Fatalf("Expected 1 VolumeClaimTemplate, got %d", len(sts.Spec.VolumeClaimTemplates))
	}

	pvc := sts.Spec.VolumeClaimTemplates[0]
	if pvc.Name != "seata-store" {
		t.Errorf("Expected PVC name 'seata-store', got '%s'", pvc.Name)
	}

	if pvc.Labels["app"] != "test-seata" {
		t.Errorf("Expected PVC label app='test-seata', got '%s'", pvc.Labels["app"])
	}

	if pvc.Labels["uid"] != "test-uid-123" {
		t.Errorf("Expected PVC label uid='test-uid-123', got '%s'", pvc.Labels["uid"])
	}

	// Check container
	if len(sts.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(sts.Spec.Template.Spec.Containers))
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if container.Name != "seata-server" {
		t.Errorf("Expected container name 'seata-server', got '%s'", container.Name)
	}

	if container.Image != "apache/seata-server:latest" {
		t.Errorf("Expected image 'apache/seata-server:latest', got '%s'", container.Image)
	}

	// Check ports
	if len(container.Ports) != 3 {
		t.Fatalf("Expected 3 container ports, got %d", len(container.Ports))
	}

	// Check volume mounts
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got %d", len(container.VolumeMounts))
	}

	if container.VolumeMounts[0].Name != "seata-store" {
		t.Errorf("Expected volume mount name 'seata-store', got '%s'", container.VolumeMounts[0].Name)
	}

	if container.VolumeMounts[0].MountPath != "/seata-server/sessionStore" {
		t.Errorf("Expected mount path '/seata-server/sessionStore', got '%s'", container.VolumeMounts[0].MountPath)
	}
}

func TestBuildEntrypointScript(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-cluster",
		},
	}

	script := buildEntrypointScript(seataServer)

	if !strings.Contains(script, "SEATA_IP=$(HOST_NAME).seata-cluster") {
		t.Error("Script should contain SEATA_IP export")
	}

	if !strings.Contains(script, "python3 -c") {
		t.Error("Script should contain python3 command")
	}

	if !strings.Contains(script, "/seata-server-entrypoint.sh") {
		t.Error("Script should contain seata-server-entrypoint.sh")
	}
}

func TestBuildEnvVars(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				Env: []apiv1.EnvVar{
					{Name: "CUSTOM_ENV", Value: "custom-value"},
				},
			},
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	envs := buildEnvVars(seataServer)

	// Check that we have at least the base envs + custom env
	if len(envs) < 5 {
		t.Errorf("Expected at least 5 env vars, got %d", len(envs))
	}

	// Create a map for easier checking
	envMap := make(map[string]string)
	for _, env := range envs {
		if env.ValueFrom == nil {
			envMap[env.Name] = env.Value
		}
	}

	// Check standard environment variables
	if envMap["store.mode"] != "raft" {
		t.Errorf("Expected store.mode='raft', got '%s'", envMap["store.mode"])
	}

	if envMap["server.port"] != "7091" {
		t.Errorf("Expected server.port='7091', got '%s'", envMap["server.port"])
	}

	if envMap["server.servicePort"] != "8091" {
		t.Errorf("Expected server.servicePort='8091', got '%s'", envMap["server.servicePort"])
	}

	// Check custom env
	if envMap["CUSTOM_ENV"] != "custom-value" {
		t.Errorf("Expected CUSTOM_ENV='custom-value', got '%s'", envMap["CUSTOM_ENV"])
	}

	// Check HOST_NAME has ValueFrom
	hasHostName := false
	for _, env := range envs {
		if env.Name == "HOST_NAME" && env.ValueFrom != nil {
			hasHostName = true
			if env.ValueFrom.FieldRef.FieldPath != "metadata.name" {
				t.Errorf("Expected HOST_NAME field path 'metadata.name', got '%s'", env.ValueFrom.FieldRef.FieldPath)
			}
		}
	}
	if !hasHostName {
		t.Error("Expected HOST_NAME environment variable with ValueFrom")
	}

	// Check server.raft.serverAddr exists
	if _, exists := envMap["server.raft.serverAddr"]; !exists {
		t.Error("Expected server.raft.serverAddr environment variable")
	}
}

func TestBuildEnvVars_WithMultipleReplicas(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 3,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
			ServiceName: "seata-cluster",
		},
	}

	envs := buildEnvVars(seataServer)

	envMap := make(map[string]string)
	for _, env := range envs {
		if env.ValueFrom == nil {
			envMap[env.Name] = env.Value
		}
	}

	raftAddr := envMap["server.raft.serverAddr"]
	// Should contain all 3 replicas in the address
	expectedSubstrings := []string{
		"test-seata-0.seata-cluster:9091",
		"test-seata-1.seata-cluster:9091",
		"test-seata-2.seata-cluster:9091",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(raftAddr, substr) {
			t.Errorf("Expected raft address to contain '%s', got '%s'", substr, raftAddr)
		}
	}
}

