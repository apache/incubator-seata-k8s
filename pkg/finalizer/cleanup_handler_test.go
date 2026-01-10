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

package finalizer

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seatav1 "github.com/apache/seata-k8s/api/v1"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
)

func TestNewCleanupHandler(t *testing.T) {
	scheme := createCleanupTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logr.Discard()

	handler := NewCleanupHandler(fakeClient, logger)

	if handler == nil {
		t.Fatal("NewCleanupHandler should not return nil")
	}
	if handler.Client == nil {
		t.Error("CleanupHandler.Client should not be nil")
	}
}

func TestHandleCleanup_V1(t *testing.T) {
	scheme := createCleanupTestScheme()
	
	seataServer := &seatav1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1.SeataServerSpec{
			Persistence: seatav1.Persistence{
				VolumeReclaimPolicy: seatav1.VolumeReclaimPolicyDelete,
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

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.HandleCleanup(context.Background(), seataServer)
	if err != nil {
		t.Errorf("HandleCleanup failed: %v", err)
	}
}

func TestHandleCleanup_V1Alpha1(t *testing.T) {
	scheme := createCleanupTestScheme()
	
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Persistence: seatav1alpha1.Persistence{
				VolumeReclaimPolicy: seatav1alpha1.VolumeReclaimPolicyDelete,
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

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.HandleCleanup(context.Background(), seataServer)
	if err != nil {
		t.Errorf("HandleCleanup failed: %v", err)
	}
}

func TestHandleCleanup_UnsupportedType(t *testing.T) {
	scheme := createCleanupTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	// Pass an unsupported type
	err := handler.HandleCleanup(context.Background(), "unsupported")
	if err == nil {
		t.Error("HandleCleanup should return error for unsupported type")
	}
}

func TestHandleCleanup_V1_RetainPolicy(t *testing.T) {
	scheme := createCleanupTestScheme()
	
	seataServer := &seatav1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: seatav1.SeataServerSpec{
			Persistence: seatav1.Persistence{
				VolumeReclaimPolicy: seatav1.VolumeReclaimPolicyRetain,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.HandleCleanup(context.Background(), seataServer)
	if err != nil {
		t.Errorf("HandleCleanup with Retain policy failed: %v", err)
	}
}

func TestCleanupAllPVCs(t *testing.T) {
	scheme := createCleanupTestScheme()

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

	pvc2 := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-2",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
				"uid": "test-uid-123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc1, pvc2).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.cleanupAllPVCs(context.Background(), "test-seata", "default", types.UID("test-uid-123"))
	if err != nil {
		t.Errorf("cleanupAllPVCs failed: %v", err)
	}
}

func TestGetPVCList(t *testing.T) {
	scheme := createCleanupTestScheme()

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
		WithObjects(pvc).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	pvcList, err := handler.getPVCList(context.Background(), "test-seata", "default", types.UID("test-uid-123"))
	if err != nil {
		t.Errorf("getPVCList failed: %v", err)
	}

	if len(pvcList.Items) != 1 {
		t.Errorf("Expected 1 PVC, got %d", len(pvcList.Items))
	}
}

func TestDeletePVC(t *testing.T) {
	scheme := createCleanupTestScheme()

	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.deletePVC(context.Background(), pvc)
	if err != nil {
		t.Errorf("deletePVC failed: %v", err)
	}
}

func TestCleanupConfigMaps(t *testing.T) {
	scheme := createCleanupTestScheme()

	cm := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cm).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.cleanupConfigMaps(context.Background(), "test-seata", "default")
	if err != nil {
		t.Errorf("cleanupConfigMaps failed: %v", err)
	}
}

func TestCleanupSecrets(t *testing.T) {
	scheme := createCleanupTestScheme()

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-seata",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	handler := NewCleanupHandler(fakeClient, logr.Discard())

	err := handler.cleanupSecrets(context.Background(), "test-seata", "default")
	if err != nil {
		t.Errorf("cleanupSecrets failed: %v", err)
	}
}

func createCleanupTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = apiv1.AddToScheme(scheme)
	_ = seatav1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)
	return scheme
}

