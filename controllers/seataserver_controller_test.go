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

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
)

func TestSeataServerReconciler_Reconcile(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
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
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyRetain,
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		WithStatusSubresource(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Log("Result requested requeue, which is expected for unsynchronized servers")
	}
}

func TestSeataServerReconciler_Reconcile_NotFound(t *testing.T) {
	scheme := createTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile should not return error for not found: %v", err)
	}

	if result.Requeue {
		t.Error("Result should not request requeue for not found")
	}
}

func TestSeataServerReconciler_ReconcileHeadlessService(t *testing.T) {
	scheme := createTestScheme()

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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileHeadlessService(context.Background(), seataServer)
	if err != nil {
		t.Errorf("reconcileHeadlessService failed: %v", err)
	}

	// Verify service was created
	svc := &apiv1.Service{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "seata-cluster",
		Namespace: "default",
	}, svc)

	if err != nil {
		t.Errorf("Failed to get created service: %v", err)
	}

	if svc.Spec.ClusterIP != "None" {
		t.Errorf("Expected headless service (ClusterIP=None), got ClusterIP=%s", svc.Spec.ClusterIP)
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		WithStatusSubresource(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err != nil {
		t.Errorf("reconcileStatefulSet failed: %v", err)
	}

	// Verify StatefulSet was created
	sts := &appsv1.StatefulSet{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-seata",
		Namespace: "default",
	}, sts)

	if err != nil {
		t.Errorf("Failed to get created StatefulSet: %v", err)
	}

	if *sts.Spec.Replicas != 3 {
		t.Errorf("Expected 3 replicas, got %d", *sts.Spec.Replicas)
	}
}

func TestSeataServerReconciler_UpdateStatefulSet(t *testing.T) {
	scheme := createTestScheme()
	replicas3 := int32(3)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:v2",
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

	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "seata-server",
							Image: "apache/seata-server:v1",
						},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSts).
		WithStatusSubresource(seataServer, existingSts).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err != nil {
		t.Errorf("reconcileStatefulSet failed during update: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_Delete(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-seata",
			Namespace:         "default",
			UID:               types.UID("test-uid-123"),
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
			Finalizers:        []string{"cleanUpSeataPVC"},
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
			},
		},
	}

	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileFinalizers(context.Background(), seataServer)
	if err != nil {
		t.Errorf("reconcileFinalizers failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_Retain(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyRetain,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileFinalizers(context.Background(), seataServer)
	if err != nil {
		t.Errorf("reconcileFinalizers with Retain policy failed: %v", err)
	}
}

func TestSeataServerReconciler_CleanupOrphanPVCs(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 2,
		},
	}

	// PVC 0 and 1 are in use, PVC 2 is orphan
	pvc0 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	pvc1 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	pvc2 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-2",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc0, pvc1, pvc2).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.cleanupOrphanPVCs(context.Background(), seataServer)
	if err != nil {
		t.Errorf("cleanupOrphanPVCs failed: %v", err)
	}
}

func TestSeataServerReconciler_GetPVCList(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
		},
	}

	pvc0 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	pvc1 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc0, pvc1).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	pvcList, err := reconciler.getPVCList(context.Background(), seataServer)
	if err != nil {
		t.Errorf("getPVCList failed: %v", err)
	}

	if len(pvcList.Items) != 2 {
		t.Errorf("Expected 2 PVCs, got %d", len(pvcList.Items))
	}
}

func TestSeataServerReconciler_GetPVCCount(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
	}

	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	count, err := reconciler.getPVCCount(context.Background(), seataServer)
	if err != nil {
		t.Errorf("getPVCCount failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected PVC count 1, got %d", count)
	}
}

func TestSeataServerReconciler_RecordError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Status: seatav1alpha1.SeataServerStatus{
			Errors: []seatav1alpha1.SeataServerError{},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		WithStatusSubresource(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.recordError(
		context.Background(),
		types.NamespacedName{Name: "test-seata", Namespace: "default"},
		seatav1alpha1.ErrorTypeK8s_SeataServer,
		"Test error message",
		reconcile.TerminalError(nil),
	)

	if err != nil {
		t.Errorf("recordError failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileClientObject(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "test-service",
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileClientObject(
		context.Background(),
		seataServer,
		svc,
		func() client.Object { return &apiv1.Service{} },
		func(found, desired client.Object) {},
		seatav1alpha1.ErrorTypeK8s_HeadlessService,
		"Service",
	)

	if err != nil {
		t.Errorf("reconcileClientObject failed: %v", err)
	}

	// Verify service was created
	createdSvc := &apiv1.Service{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-service",
		Namespace: "default",
	}, createdSvc)

	if err != nil {
		t.Errorf("Failed to get created service: %v", err)
	}
}

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = apiv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)
	return scheme
}

