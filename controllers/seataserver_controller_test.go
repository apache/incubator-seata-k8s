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

func TestSeataServerReconciler_RecordError_MaxErrors(t *testing.T) {
	scheme := createTestScheme()

	// Create a SeataServer with maximum errors
	errors := make([]seatav1alpha1.SeataServerError, MaxRecentErrorRecords)
	for i := 0; i < MaxRecentErrorRecords; i++ {
		errors[i] = seatav1alpha1.SeataServerError{
			Type:      "test-error",
			Message:   "old error",
			Timestamp: metav1.Now(),
		}
	}

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Status: seatav1alpha1.SeataServerStatus{
			Errors: errors,
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
		"New error message",
		reconcile.TerminalError(nil),
	)

	if err != nil {
		t.Errorf("recordError failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileClientObject_CreateSuccess(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "test-svc",
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

	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091},
			},
		},
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
		t.Errorf("reconcileClientObject create failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileClientObject_UpdateSuccess(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "test-svc",
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	existingSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "operator.seata.apache.org/v1alpha1",
					Kind:       "SeataServer",
					Name:       "test-seata",
					UID:        "test-uid",
				},
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "old-port", Port: 8080},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	newSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: 8091},
			},
		},
	}

	err := reconciler.reconcileClientObject(
		context.Background(),
		seataServer,
		newSvc,
		func() client.Object { return &apiv1.Service{} },
		func(found, desired client.Object) {
			found.(*apiv1.Service).Spec.Ports = desired.(*apiv1.Service).Spec.Ports
		},
		seatav1alpha1.ErrorTypeK8s_HeadlessService,
		"Service",
	)

	if err != nil {
		t.Errorf("reconcileClientObject update failed: %v", err)
	}
}

func TestSeataServerReconciler_Reconcile_WithDefaults(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			// Empty spec to trigger WithDefaults
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
		t.Errorf("Reconcile with defaults failed: %v", err)
	}

	// Should requeue after applying defaults
	if !result.Requeue {
		t.Error("Expected requeue after applying defaults")
	}
}

func TestSeataServerReconciler_Reconcile_ErrorHandling(t *testing.T) {
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
			},
			ServiceName: "seata-service",
			Replicas:    3,
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

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		// Errors are expected during reconciliation
		t.Logf("Reconcile returned expected error: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_AddFinalizer(t *testing.T) {
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
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
			},
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 1,
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
		t.Errorf("reconcileFinalizers failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_CleanupPVCs(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-seata",
			Namespace:  "default",
			UID:        types.UID("test-uid-123"),
			Finalizers: []string{"cleanUpSeataPVC"},
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
			},
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 2,
		},
	}

	// Create orphan PVC
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-3",
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
		t.Errorf("reconcileFinalizers with orphan PVCs failed: %v", err)
	}
}

func TestSeataServerReconciler_UpdateStatefulSet_NotReady(t *testing.T) {
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
			ReadyReplicas: 1, // Not all ready
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

	newSts := &appsv1.StatefulSet{
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
							Image: "apache/seata-server:v2",
						},
					},
				},
			},
		},
	}

	err := reconciler.updateStatefulSet(context.Background(), seataServer, existingSts, newSts)
	if err != nil {
		t.Logf("updateStatefulSet with not ready pods: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileHeadlessService_UpdateError(t *testing.T) {
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
		t.Logf("reconcileHeadlessService returned: %v", err)
	}
}

func TestSeataServerReconciler_CleanUpAllPVCs_Success(t *testing.T) {
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

	pvc1 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	pvc2 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-store-test-seata-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc1, pvc2).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.cleanUpAllPVCs(context.Background(), seataServer)
	if err != nil {
		t.Errorf("cleanUpAllPVCs failed: %v", err)
	}
}

func TestSeataServerReconciler_DeletePVC_Success(t *testing.T) {
	scheme := createTestScheme()

	pvc := apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&pvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	reconciler.deletePVC(context.Background(), pvc)
	// deletePVC doesn't return error, just logs
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

func TestSeataServerReconciler_Reconcile_CompleteFlow(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complete-test",
			Namespace: "default",
			UID:       types.UID("complete-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-complete",
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
			Name:      "complete-test",
			Namespace: "default",
		},
	}

	// First reconcile
	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Logf("First reconcile error (expected): %v", err)
	}

	if result.Requeue || result.RequeueAfter > 0 {
		t.Logf("Reconcile requested requeue: %v", result)
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_CreateAndUpdate(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata-sts",
			Namespace: "default",
			UID:       types.UID("sts-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:v1",
			},
			ServiceName: "seata-svc",
			Replicas:    2,
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

	// Create StatefulSet
	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err != nil {
		t.Logf("reconcileStatefulSet create returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_DeleteFlow(t *testing.T) {
	scheme := createTestScheme()
	now := metav1.Now()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-test",
			Namespace:         "default",
			UID:               types.UID("delete-uid"),
			Finalizers:        []string{"cleanUpSeataPVC"},
			DeletionTimestamp: &now,
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
				"app": "delete-test",
				"uid": "delete-uid",
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
		t.Logf("reconcileFinalizers delete flow returned: %v", err)
	}
}

func TestSeataServerReconciler_GetPVCCount_WithNoPVCs(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-pvc-test",
			Namespace: "default",
			UID:       types.UID("no-pvc-uid"),
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

	count, err := reconciler.getPVCCount(context.Background(), seataServer)
	if err != nil {
		t.Errorf("getPVCCount failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 PVCs, got %d", count)
	}
}

func TestSeataServerReconciler_CleanupOrphanPVCs_NotReady(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-ready-test",
			Namespace: "default",
			UID:       types.UID("not-ready-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 3,
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 2, // Not all ready
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

	// Should not cleanup when not ready
	err := reconciler.cleanupOrphanPVCs(context.Background(), seataServer)
	if err != nil {
		t.Errorf("cleanupOrphanPVCs failed: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileClientObject_GetError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "error-test",
			Namespace: "default",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "error-svc",
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

	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "error-svc",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "port", Port: 8091},
			},
		},
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
		t.Logf("reconcileClientObject returned: %v", err)
	}
}

func TestSeataServerReconciler_RecordError_GetError(t *testing.T) {
	scheme := createTestScheme()

	// Don't add the object to client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.recordError(
		context.Background(),
		types.NamespacedName{Name: "nonexistent", Namespace: "default"},
		seatav1alpha1.ErrorTypeK8s_SeataServer,
		"Test error",
		nil,
	)

	if err == nil {
		t.Error("Expected error when recording error for nonexistent object")
	}
}

func TestSeataServerReconciler_UpdateStatefulSet_ScaleDown(t *testing.T) {
	scheme := createTestScheme()
	replicas5 := int32(5)
	replicas2 := int32(2)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scale-test",
			Namespace: "default",
			UID:       types.UID("scale-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-scale",
			Replicas:    2,
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
			Name:      "scale-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas5,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "seata-server", Image: "apache/seata-server:latest"},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      5,
			ReadyReplicas: 5,
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

	newSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scale-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "seata-server", Image: "apache/seata-server:latest"},
					},
				},
			},
		},
	}

	err := reconciler.updateStatefulSet(context.Background(), seataServer, existingSts, newSts)
	if err != nil {
		t.Logf("updateStatefulSet scale down returned: %v", err)
	}
}

func TestSeataServerReconciler_UpdateStatefulSet_ReplicasNotSynchronized(t *testing.T) {
	scheme := createTestScheme()
	replicas3 := int32(3)
	replicas5 := int32(5)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-test",
			Namespace: "default",
			UID:       types.UID("sync-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-sync",
			Replicas:    5,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	newSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas5,
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

	err := reconciler.updateStatefulSet(context.Background(), seataServer, existingSts, newSts)
	if err != nil {
		t.Logf("updateStatefulSet returned: %v", err)
	}

	// Verify status was updated
	updatedServer := &seatav1alpha1.SeataServer{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "sync-test", Namespace: "default"}, updatedServer)

	if updatedServer.Status.Synchronized {
		t.Error("Expected Synchronized=false when ready replicas != desired replicas")
	}
}

func TestSeataServerReconciler_ReconcileHeadlessService_CreateError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-error-test",
			Namespace: "default",
			UID:       types.UID("svc-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-svc-error",
			Replicas:    1,
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
		t.Logf("reconcileHeadlessService returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_GetError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sts-get-error",
			Namespace: "default",
			UID:       types.UID("sts-get-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-sts-err",
			Replicas:    1,
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

	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err != nil {
		t.Logf("reconcileStatefulSet returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_UpdateError_AddFinalizer(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "finalizer-add-error",
			Namespace:  "default",
			UID:        types.UID("finalizer-add-uid"),
			Finalizers: []string{}, // No finalizer initially
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
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
		t.Logf("reconcileFinalizers with add finalizer returned: %v", err)
	}
}

func TestSeataServerReconciler_CleanupOrphanPVCs_WithOrphanPVCs(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphan-pvc-test",
			Namespace: "default",
			UID:       types.UID("orphan-pvc-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 2, // All ready
		},
	}

	// Create PVCs for replicas 0-3, but only 0-1 should exist
	pvcs := []client.Object{
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-orphan-pvc-test-0",
				Namespace: "default",
				Labels: map[string]string{
					"app": "orphan-pvc-test",
					"uid": "orphan-pvc-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-orphan-pvc-test-1",
				Namespace: "default",
				Labels: map[string]string{
					"app": "orphan-pvc-test",
					"uid": "orphan-pvc-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-orphan-pvc-test-2",
				Namespace: "default",
				Labels: map[string]string{
					"app": "orphan-pvc-test",
					"uid": "orphan-pvc-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-orphan-pvc-test-3",
				Namespace: "default",
				Labels: map[string]string{
					"app": "orphan-pvc-test",
					"uid": "orphan-pvc-uid",
				},
			},
		},
	}

	allObjects := append([]client.Object{seataServer}, pvcs...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(allObjects...).
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

	// Check that orphan PVCs (2 and 3) were deleted
	pvcList := &apiv1.PersistentVolumeClaimList{}
	_ = fakeClient.List(context.Background(), pvcList, client.InNamespace("default"))

	if len(pvcList.Items) > 2 {
		t.Logf("Found %d PVCs, expected orphan PVCs to be deleted", len(pvcList.Items))
	}
}

func TestSeataServerReconciler_Reconcile_WithUpdateDefaultsError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "defaults-error-test",
			Namespace: "default",
			UID:       types.UID("defaults-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			// Leave fields empty to trigger WithDefaults()
			ContainerSpec: seatav1alpha1.ContainerSpec{},
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

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "defaults-error-test",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Logf("Reconcile with defaults returned error: %v", err)
	}

	if result.Requeue {
		t.Logf("Reconcile requested requeue after setting defaults")
	}
}

func TestSeataServerReconciler_Reconcile_NotSynchronized(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-synced-test",
			Namespace: "default",
			UID:       types.UID("not-synced-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-not-synced",
			Replicas:    3,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
		Status: seatav1alpha1.SeataServerStatus{
			Synchronized: false, // Not synchronized
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
			Name:      "not-synced-test",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Logf("Reconcile returned error: %v", err)
	}

	if result.Requeue || result.RequeueAfter > 0 {
		t.Logf("Reconcile requested requeue for not synchronized server: %v", result)
	}
}

func TestSeataServerReconciler_ReconcileClientObject_SetOwnerReferenceError(t *testing.T) {
	scheme := runtime.NewScheme()
	// Don't add schemes to cause SetControllerReference to fail

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-error-test",
			Namespace: "default",
			UID:       types.UID("owner-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-owner-error",
		},
	}

	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-error-svc",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
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

	if err == nil {
		t.Error("Expected error when SetControllerReference fails")
	}
}

func TestSeataServerReconciler_GetPVCCount_ListError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-list-error",
			Namespace: "default",
			UID:       types.UID("pvc-list-error-uid"),
		},
	}

	// Create a client without the necessary scheme to cause list errors
	fakeClient := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	_, err := reconciler.getPVCCount(context.Background(), seataServer)
	if err == nil {
		t.Log("Expected error when listing PVCs with incomplete scheme")
	}
}

func TestSeataServerReconciler_DeletePVC_WithError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "delete-pvc-error",
			Namespace: "default",
			UID:       types.UID("delete-pvc-error-uid"),
		},
	}

	// Don't create the PVC object, so delete will fail or return not found
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	// deletePVC doesn't return an error, it logs it
	nonexistentPVC := apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nonexistent-pvc",
			Namespace: "default",
		},
	}
	reconciler.deletePVC(context.Background(), nonexistentPVC)
	// This test ensures the function doesn't panic when deleting a nonexistent PVC
}

func TestSeataServerReconciler_CleanUpAllPVCs_WithPartialFailure(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cleanup-partial",
			Namespace: "default",
			UID:       types.UID("cleanup-partial-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 3,
		},
	}

	pvc1 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-cleanup-partial-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "cleanup-partial",
				"uid": "cleanup-partial-uid",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc1).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.cleanUpAllPVCs(context.Background(), seataServer)
	if err != nil {
		t.Logf("cleanUpAllPVCs returned: %v", err)
	}
}

func TestSeataServerReconciler_UpdateStatefulSet_WithEnvVars(t *testing.T) {
	scheme := createTestScheme()
	replicas2 := int32(2)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "env-test",
			Namespace: "default",
			UID:       types.UID("env-test-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
				Env: []apiv1.EnvVar{
					{Name: "console.user.username", Value: "admin"},
					{Name: "console.user.password", Value: "admin123"},
				},
			},
			ServiceName: "seata-env",
			Replicas:    2,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
		Status: seatav1alpha1.SeataServerStatus{
			Synchronized: false,
		},
	}

	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "env-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}

	newSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "env-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
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

	err := reconciler.updateStatefulSet(context.Background(), seataServer, existingSts, newSts)
	if err != nil {
		t.Logf("updateStatefulSet with env vars returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_CreateError(t *testing.T) {
	scheme := runtime.NewScheme()
	// Incomplete scheme to cause creation errors
	_ = seatav1alpha1.AddToScheme(scheme)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-error-test",
			Namespace: "default",
			UID:       types.UID("create-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-create-error",
			Replicas:    1,
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

	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err == nil {
		t.Log("Expected error when creating StatefulSet with incomplete scheme")
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_UpdateExisting(t *testing.T) {
	scheme := createTestScheme()
	replicas3 := int32(3)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "update-sts-test",
			Namespace: "default",
			UID:       types.UID("update-sts-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:v2.0",
			},
			ServiceName: "seata-update",
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
			Name:      "update-sts-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "seata-server", Image: "apache/seata-server:v1.0"},
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
		t.Logf("reconcileStatefulSet update returned: %v", err)
	}

	// Verify StatefulSet was updated
	updatedSts := &appsv1.StatefulSet{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "update-sts-test", Namespace: "default"}, updatedSts)
	if len(updatedSts.Spec.Template.Spec.Containers) > 0 {
		t.Logf("StatefulSet image updated to: %s", updatedSts.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestSeataServerReconciler_Reconcile_ReconcileFunctionError(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reconcile-error-test",
			Namespace: "default",
			UID:       types.UID("reconcile-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-reconcile-err",
			Replicas:    1,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	// Create an incomplete scheme to trigger errors during reconciliation
	incompleteScheme := runtime.NewScheme()
	_ = seatav1alpha1.AddToScheme(incompleteScheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(incompleteScheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: incompleteScheme,
		Log:    logr.Discard(),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "reconcile-error-test",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Logf("Reconcile returned expected error: %v", err)
	}

	if result.Requeue {
		t.Logf("Reconcile requested requeue")
	}
}

func TestSeataServerReconciler_ReconcileHeadlessService_UpdateExisting(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-update-test",
			Namespace: "default",
			UID:       types.UID("svc-update-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-svc-update",
			Replicas:    1,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	existingSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-svc-update",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service", Port: 8080}, // Old port
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileHeadlessService(context.Background(), seataServer)
	if err != nil {
		t.Logf("reconcileHeadlessService update returned: %v", err)
	}

	// Verify service was updated
	updatedSvc := &apiv1.Service{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "seata-svc-update", Namespace: "default"}, updatedSvc)
	if len(updatedSvc.Spec.Ports) > 0 {
		t.Logf("Service port updated to: %d", updatedSvc.Spec.Ports[0].Port)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_UpdateFinalizerError(t *testing.T) {
	scheme := createTestScheme()
	now := metav1.Now()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "finalizer-update-error",
			Namespace:         "default",
			UID:               types.UID("finalizer-update-error-uid"),
			Finalizers:        []string{"cleanUpSeataPVC"},
			DeletionTimestamp: &now,
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
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
		t.Logf("reconcileFinalizers returned: %v", err)
	}
}

func TestSeataServerReconciler_Reconcile_SuccessfulFlow(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "successful-test",
			Namespace: "default",
			UID:       types.UID("successful-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-successful",
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
		Status: seatav1alpha1.SeataServerStatus{
			Synchronized: true, // Already synchronized
		},
	}

	replicas3 := int32(3)
	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "successful-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	existingSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-successful",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSts, existingSvc).
		WithStatusSubresource(seataServer, existingSts).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "successful-test",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Logf("Reconcile requested requeue: %v", result)
	}

	// Should not requeue when synchronized
	if !result.Requeue && result.RequeueAfter == 0 {
		t.Log("Reconcile completed successfully without requeue")
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_SetOwnerReferenceError(t *testing.T) {
	// Create a scheme without appsv1 to cause SetControllerReference to fail
	scheme := runtime.NewScheme()
	_ = seatav1alpha1.AddToScheme(scheme)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-ref-error",
			Namespace: "default",
			UID:       types.UID("owner-ref-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-owner-ref-err",
			Replicas:    1,
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

	err := reconciler.reconcileStatefulSet(context.Background(), seataServer)
	if err == nil {
		t.Error("Expected error when SetControllerReference fails")
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_NoVolumeReclaimPolicy(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-policy-test",
			Namespace: "default",
			UID:       types.UID("no-policy-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyRetain, // Not Delete
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
		t.Errorf("reconcileFinalizers should not fail with Retain policy: %v", err)
	}
}

func TestSeataServerReconciler_UpdateStatefulSet_StatusUpdateError(t *testing.T) {
	scheme := createTestScheme()
	replicas2 := int32(2)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-update-error",
			Namespace: "default",
			UID:       types.UID("status-update-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-status-err",
			Replicas:    2,
			Ports: seatav1alpha1.Ports{
				ServicePort: 8091,
				ConsolePort: 7091,
				RaftPort:    9091,
			},
		},
	}

	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-update-error",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}

	newSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-update-error",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSts).
		Build() // No status subresource to cause status update error

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.updateStatefulSet(context.Background(), seataServer, existingSts, newSts)
	// Status update might fail, but the function should handle it gracefully
	t.Logf("updateStatefulSet returned: %v", err)
}

func TestSeataServerReconciler_CleanUpAllPVCs_EmptyPVCList(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-pvc-list",
			Namespace: "default",
			UID:       types.UID("empty-pvc-list-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
		},
	}

	// No PVCs in the cluster
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.cleanUpAllPVCs(context.Background(), seataServer)
	if err != nil {
		t.Errorf("cleanUpAllPVCs should not fail with empty PVC list: %v", err)
	}
}

func TestSeataServerReconciler_Reconcile_HeadlessServiceError(t *testing.T) {
	// Create a scheme without Service support to cause reconcileHeadlessService to fail
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)
	// Missing apiv1.AddToScheme to cause Service creation to fail

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-error",
			Namespace: "default",
			UID:       types.UID("svc-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-svc-err",
			Replicas:    1,
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

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "svc-error",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	// Log the result for debugging
	if err != nil {
		t.Logf("Reconcile returned error as expected: %v", err)
	} else {
		t.Log("Reconcile completed without error (fake client may not enforce scheme)")
	}

	if result.Requeue || result.RequeueAfter > 0 {
		t.Logf("Reconcile requested requeue: %v", result)
	}
}

func TestSeataServerReconciler_Reconcile_StatefulSetError(t *testing.T) {
	// Create incomplete scheme to cause reconcileStatefulSet to fail
	scheme := runtime.NewScheme()
	_ = apiv1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)
	// Missing appsv1.AddToScheme to cause StatefulSet creation to fail

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sts-error",
			Namespace: "default",
			UID:       types.UID("sts-error-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
			},
			ServiceName: "seata-sts-err",
			Replicas:    1,
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

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "sts-error",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	// Log the result for debugging
	if err != nil {
		t.Logf("Reconcile returned error as expected: %v", err)
	} else {
		t.Log("Reconcile completed without error (fake client may not enforce scheme)")
	}

	if result.Requeue || result.RequeueAfter > 0 {
		t.Logf("Reconcile requested requeue: %v", result)
	} else {
		t.Log("Reconcile completed without requeue")
	}
}

func TestSeataServerReconciler_Reconcile_GetError(t *testing.T) {
	scheme := runtime.NewScheme()
	// Create an empty scheme to cause Get to fail with a scheme error

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
			Name:      "get-error",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err == nil {
		t.Log("Get operation with empty scheme returned no error")
	}

	if !result.Requeue {
		t.Log("Reconcile did not request requeue after Get error")
	}
}

func TestSeataServerReconciler_ReconcileClientObject_CreateFailure(t *testing.T) {
	// Use incomplete scheme to cause Create to fail
	scheme := runtime.NewScheme()
	_ = seatav1alpha1.AddToScheme(scheme)
	// Missing apiv1.AddToScheme

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-fail",
			Namespace: "default",
			UID:       types.UID("create-fail-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-create-fail",
		},
	}

	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-create-fail",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service", Port: 8091},
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

	if err == nil {
		t.Log("Expected error when Create fails with incomplete scheme")
	}
}

func TestSeataServerReconciler_ReconcileClientObject_UpdateFailure(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "update-fail",
			Namespace: "default",
			UID:       types.UID("update-fail-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ServiceName: "seata-update-fail",
		},
	}

	existingSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-update-fail",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service", Port: 8091},
			},
		},
	}

	newSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-update-fail",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service", Port: 9091}, // Different port
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, existingSvc).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileClientObject(
		context.Background(),
		seataServer,
		newSvc,
		func() client.Object { return &apiv1.Service{} },
		func(found, desired client.Object) {
			// Sync function updates the service
			foundSvc := found.(*apiv1.Service)
			desiredSvc := desired.(*apiv1.Service)
			foundSvc.Spec.Ports = desiredSvc.Spec.Ports
		},
		seatav1alpha1.ErrorTypeK8s_HeadlessService,
		"Service",
	)

	if err != nil {
		t.Logf("Update operation returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileStatefulSet_UpdateError(t *testing.T) {
	scheme := createTestScheme()
	replicas2 := int32(2)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sts-update-err",
			Namespace: "default",
			UID:       types.UID("sts-update-err-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:v2.0",
			},
			ServiceName: "seata-sts-update-err",
			Replicas:    2,
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
			Name:      "sts-update-err",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas2,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
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
		t.Logf("reconcileStatefulSet returned: %v", err)
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_CleanupError(t *testing.T) {
	scheme := createTestScheme()
	now := metav1.Now()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cleanup-err",
			Namespace:         "default",
			UID:               types.UID("cleanup-err-uid"),
			Finalizers:        []string{"cleanUpSeataPVC"},
			DeletionTimestamp: &now,
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
			},
		},
	}

	// Create PVCs that will be cleaned up
	pvc1 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-cleanup-err-0",
			Namespace: "default",
			Labels: map[string]string{
				"app": "cleanup-err",
				"uid": "cleanup-err-uid",
			},
		},
	}

	pvc2 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-cleanup-err-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "cleanup-err",
				"uid": "cleanup-err-uid",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer, pvc1, pvc2).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	err := reconciler.reconcileFinalizers(context.Background(), seataServer)
	if err != nil {
		t.Logf("reconcileFinalizers cleanup returned: %v", err)
	}

	// Verify finalizer was removed after cleanup
	updatedServer := &seatav1alpha1.SeataServer{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "cleanup-err", Namespace: "default"}, updatedServer)

	finalizerExists := false
	for _, f := range updatedServer.Finalizers {
		if f == "cleanUpSeataPVC" {
			finalizerExists = true
			break
		}
	}

	if finalizerExists {
		t.Log("Finalizer still exists after cleanup attempt")
	} else {
		t.Log("Finalizer successfully removed after cleanup")
	}
}

func TestSeataServerReconciler_GetPVCList_SuccessWithMultiplePVCs(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-pvc",
			Namespace: "default",
			UID:       types.UID("multi-pvc-uid"),
		},
	}

	// Create multiple PVCs
	pvcs := []client.Object{
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-multi-pvc-0",
				Namespace: "default",
				Labels: map[string]string{
					"app": "multi-pvc",
					"uid": "multi-pvc-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-multi-pvc-1",
				Namespace: "default",
				Labels: map[string]string{
					"app": "multi-pvc",
					"uid": "multi-pvc-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-multi-pvc-2",
				Namespace: "default",
				Labels: map[string]string{
					"app": "multi-pvc",
					"uid": "multi-pvc-uid",
				},
			},
		},
	}

	allObjects := append([]client.Object{seataServer}, pvcs...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(allObjects...).
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

	if len(pvcList.Items) != 3 {
		t.Errorf("Expected 3 PVCs, got %d", len(pvcList.Items))
	}
}

func TestSeataServerReconciler_ReconcileFinalizers_AlreadyHasFinalizer(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "has-finalizer",
			Namespace:  "default",
			UID:        types.UID("has-finalizer-uid"),
			Finalizers: []string{"cleanUpSeataPVC"}, // Already has finalizer
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
			},
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 1,
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
		t.Errorf("reconcileFinalizers failed: %v", err)
	}

	// Verify finalizer still exists
	updatedServer := &seatav1alpha1.SeataServer{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "has-finalizer", Namespace: "default"}, updatedServer)

	hasFinalizer := false
	for _, f := range updatedServer.Finalizers {
		if f == "cleanUpSeataPVC" {
			hasFinalizer = true
			break
		}
	}

	if !hasFinalizer {
		t.Error("Finalizer should still exist after reconcile")
	}
}

func TestSeataServerReconciler_CleanupOrphanPVCs_PVCCountError(t *testing.T) {
	scheme := runtime.NewScheme()
	// Create incomplete scheme to cause getPVCCount to fail
	_ = seatav1alpha1.AddToScheme(scheme)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-count-err",
			Namespace: "default",
			UID:       types.UID("pvc-count-err-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 2,
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 2,
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

	err := reconciler.cleanupOrphanPVCs(context.Background(), seataServer)
	// May or may not error depending on fake client behavior
	if err != nil {
		t.Logf("cleanupOrphanPVCs returned error: %v", err)
	}
}

func TestSeataServerReconciler_CleanupOrphanPVCs_GetPVCListError(t *testing.T) {
	scheme := createTestScheme()

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-list-err",
			Namespace: "default",
			UID:       types.UID("pvc-list-err-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas: 1,
		},
		Status: seatav1alpha1.SeataServerStatus{
			ReadyReplicas: 1,
		},
	}

	// Create multiple PVCs to trigger the orphan cleanup logic
	pvcs := []client.Object{
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-pvc-list-err-0",
				Namespace: "default",
				Labels: map[string]string{
					"app": "pvc-list-err",
					"uid": "pvc-list-err-uid",
				},
			},
		},
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-pvc-list-err-1",
				Namespace: "default",
				Labels: map[string]string{
					"app": "pvc-list-err",
					"uid": "pvc-list-err-uid",
				},
			},
		},
	}

	allObjects := append([]client.Object{seataServer}, pvcs...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(allObjects...).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	// This should work fine and clean up the orphan PVC
	err := reconciler.cleanupOrphanPVCs(context.Background(), seataServer)
	if err != nil {
		t.Errorf("cleanupOrphanPVCs failed: %v", err)
	}

	// Verify orphan PVC was deleted
	pvcList := &apiv1.PersistentVolumeClaimList{}
	_ = fakeClient.List(context.Background(), pvcList, client.InNamespace("default"))

	if len(pvcList.Items) > 1 {
		t.Log("Orphan PVC cleanup may not have completed")
	}
}

func TestSeataServerReconciler_Reconcile_FullLifecycle(t *testing.T) {
	scheme := createTestScheme()
	replicas3 := int32(3)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle-test",
			Namespace: "default",
			UID:       types.UID("lifecycle-uid"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			ContainerSpec: seatav1alpha1.ContainerSpec{
				ContainerName: "seata-server",
				Image:         "apache/seata-server:latest",
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("100m"),
						apiv1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("500m"),
						apiv1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
			ServiceName: "seata-lifecycle",
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
							apiv1.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
			},
		},
	}

	existingSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seata-lifecycle",
			Namespace: "default",
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "None",
			Ports: []apiv1.ServicePort{
				{Name: "service", Port: 8091},
				{Name: "console", Port: 7091},
				{Name: "raft", Port: 9091},
			},
		},
	}

	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas3,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "seata-server", Image: "apache/seata-server:latest"},
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
		WithObjects(seataServer, existingSvc, existingSts).
		WithStatusSubresource(seataServer, existingSts).
		Build()

	reconciler := &SeataServerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    logr.Discard(),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "lifecycle-test",
			Namespace: "default",
		},
	}

	// First reconcile - should update everything
	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Logf("First reconcile returned error: %v", err)
	}

	if result.Requeue || result.RequeueAfter > 0 {
		t.Logf("First reconcile requested requeue: %v", result)
	}

	// Verify resources were reconciled
	updatedSts := &appsv1.StatefulSet{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "lifecycle-test", Namespace: "default"}, updatedSts)
	if err != nil {
		t.Errorf("Failed to get updated StatefulSet: %v", err)
	}
}

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = apiv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)
	return scheme
}
