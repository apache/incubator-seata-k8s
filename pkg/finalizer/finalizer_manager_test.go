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
	"time"

	seatav1 "github.com/apache/seata-k8s/api/v1"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewFinalizerManager(t *testing.T) {
	scheme := createTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logr.Discard()

	fm := NewFinalizerManager(fakeClient, logger)

	if fm == nil {
		t.Fatal("NewFinalizerManager should not return nil")
	}
	if fm.Client == nil {
		t.Error("FinalizerManager.Client should not be nil")
	}
}

func TestFinalizerManager_AddFinalizer(t *testing.T) {
	testCases := []struct {
		name        string
		seataServer *seatav1.SeataServer
		finalizer   string
		expectError bool
	}{
		{
			name: "add finalizer to seataserver without finalizers",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{},
				},
			},
			finalizer:   SeataFinalizerName,
			expectError: false,
		},
		{
			name: "add finalizer to seataserver with existing finalizers",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{"other-finalizer"},
				},
			},
			finalizer:   SeataFinalizerName,
			expectError: false,
		},
		{
			name: "add duplicate finalizer",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{SeataFinalizerName},
				},
			},
			finalizer:   SeataFinalizerName,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := createTestScheme()
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.seataServer).
				Build()

			fm := &FinalizerManager{
				Client: fakeClient,
				Log:    logr.Discard(),
			}

			ctx := context.Background()
			err := fm.AddFinalizer(ctx, tc.seataServer, tc.finalizer)

			if (err != nil) != tc.expectError {
				t.Errorf("expected error: %v, got: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if !fm.HasFinalizer(tc.seataServer, tc.finalizer) {
					t.Errorf("finalizer %s not found after adding", tc.finalizer)
				}
			}
		})
	}
}

func TestFinalizerManager_RemoveFinalizer(t *testing.T) {
	testCases := []struct {
		name        string
		seataServer *seatav1.SeataServer
		finalizer   string
		expectError bool
		expectFound bool
	}{
		{
			name: "remove existing finalizer",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{SeataFinalizerName, "other-finalizer"},
				},
			},
			finalizer:   SeataFinalizerName,
			expectError: false,
			expectFound: false,
		},
		{
			name: "remove non-existing finalizer",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{"other-finalizer"},
				},
			},
			finalizer:   SeataFinalizerName,
			expectError: false,
			expectFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := createTestScheme()
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.seataServer).
				Build()

			fm := &FinalizerManager{
				Client: fakeClient,
				Log:    logr.Discard(),
			}

			ctx := context.Background()
			err := fm.RemoveFinalizer(ctx, tc.seataServer, tc.finalizer)

			if (err != nil) != tc.expectError {
				t.Errorf("expected error: %v, got: %v", tc.expectError, err)
			}

			hasFinalizer := fm.HasFinalizer(tc.seataServer, tc.finalizer)
			if hasFinalizer != tc.expectFound {
				t.Errorf("expected finalizer found: %v, got: %v", tc.expectFound, hasFinalizer)
			}
		})
	}
}

func TestFinalizerManager_HasFinalizer(t *testing.T) {
	testCases := []struct {
		name        string
		seataServer *seatav1.SeataServer
		finalizer   string
		expected    bool
	}{
		{
			name: "finalizer exists",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{SeataFinalizerName},
				},
			},
			finalizer: SeataFinalizerName,
			expected:  true,
		},
		{
			name: "finalizer does not exist",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{"other-finalizer"},
				},
			},
			finalizer: SeataFinalizerName,
			expected:  false,
		},
		{
			name: "no finalizers",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-seata",
					Namespace:  "default",
					Finalizers: []string{},
				},
			},
			finalizer: SeataFinalizerName,
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fm := &FinalizerManager{
				Client: nil,
				Log:    logr.Discard(),
			}

			result := fm.HasFinalizer(tc.seataServer, tc.finalizer)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestFinalizerManager_IsBeingDeleted(t *testing.T) {
	testCases := []struct {
		name        string
		seataServer *seatav1.SeataServer
		expected    bool
	}{
		{
			name: "seataserver not being deleted",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-seata",
					Namespace: "default",
				},
			},
			expected: false,
		},
		{
			name: "seataserver not being deleted",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-seata",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{Time: time.Time{}},
				},
			},
			expected: false,
		},
		{
			name: "seataserver being deleted",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-seata",
					Namespace: "default",
					DeletionTimestamp: &metav1.Time{
						Time: time.Now(),
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fm := &FinalizerManager{
				Client: nil,
				Log:    logr.Discard(),
			}

			result := fm.IsBeingDeleted(tc.seataServer)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestFinalizerManager_GetNamespacedName(t *testing.T) {
	testCases := []struct {
		name        string
		server      interface{}
		expected    types.NamespacedName
	}{
		{
			name: "v1 SeataServer",
			server: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-seata",
					Namespace: "default",
				},
			},
			expected: types.NamespacedName{Name: "test-seata", Namespace: "default"},
		},
		{
			name: "v1alpha1 SeataServer",
			server: &seatav1alpha1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-alpha",
					Namespace: "test-ns",
				},
			},
			expected: types.NamespacedName{Name: "test-alpha", Namespace: "test-ns"},
		},
		{
			name:     "unsupported type",
			server:   "invalid",
			expected: types.NamespacedName{},
		},
	}

	fm := &FinalizerManager{
		Client: nil,
		Log:    logr.Discard(),
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fm.GetNamespacedName(tc.server)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestFinalizerManager_GetDeletionTimestamp(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name     string
		server   interface{}
		expected interface{}
	}{
		{
			name: "v1 SeataServer with deletion timestamp",
			server: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-seata",
					Namespace:         "default",
					DeletionTimestamp: &now,
				},
			},
			expected: &now,
		},
		{
			name: "v1alpha1 SeataServer with deletion timestamp",
			server: &seatav1alpha1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-alpha",
					Namespace:         "default",
					DeletionTimestamp: &now,
				},
			},
			expected: &now,
		},
		{
			name:     "unsupported type",
			server:   "invalid",
			expected: nil,
		},
	}

	fm := &FinalizerManager{
		Client: nil,
		Log:    logr.Discard(),
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fm.GetDeletionTimestamp(tc.server)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestFinalizerManager_AddFinalizer_V1Alpha1(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = seatav1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-seata-alpha",
			Namespace:  "default",
			Finalizers: []string{},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	fm := &FinalizerManager{
		Client: fakeClient,
		Log:    logr.Discard(),
	}

	ctx := context.Background()
	err := fm.AddFinalizer(ctx, seataServer, SeataFinalizerName)

	if err != nil {
		t.Errorf("AddFinalizer for v1alpha1 failed: %v", err)
	}

	if !fm.HasFinalizer(seataServer, SeataFinalizerName) {
		t.Errorf("finalizer %s not found after adding", SeataFinalizerName)
	}
}

func TestFinalizerManager_RemoveFinalizer_V1Alpha1(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = seatav1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-seata-alpha",
			Namespace:  "default",
			Finalizers: []string{SeataFinalizerName, "other-finalizer"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(seataServer).
		Build()

	fm := &FinalizerManager{
		Client: fakeClient,
		Log:    logr.Discard(),
	}

	ctx := context.Background()
	err := fm.RemoveFinalizer(ctx, seataServer, SeataFinalizerName)

	if err != nil {
		t.Errorf("RemoveFinalizer for v1alpha1 failed: %v", err)
	}

	hasFinalizer := fm.HasFinalizer(seataServer, SeataFinalizerName)
	if hasFinalizer {
		t.Errorf("expected finalizer to be removed")
	}
}

func TestFinalizerManager_AddFinalizer_UnsupportedType(t *testing.T) {
	scheme := createTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	fm := &FinalizerManager{
		Client: fakeClient,
		Log:    logr.Discard(),
	}

	err := fm.AddFinalizer(context.Background(), "invalid-type", SeataFinalizerName)
	if err == nil {
		t.Error("Expected error for unsupported type, got nil")
	}
}

func TestFinalizerManager_RemoveFinalizer_UnsupportedType(t *testing.T) {
	scheme := createTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	fm := &FinalizerManager{
		Client: fakeClient,
		Log:    logr.Discard(),
	}

	err := fm.RemoveFinalizer(context.Background(), "invalid-type", SeataFinalizerName)
	if err == nil {
		t.Error("Expected error for unsupported type, got nil")
	}
}

func TestFinalizerManager_HasFinalizer_UnsupportedType(t *testing.T) {
	fm := &FinalizerManager{
		Client: nil,
		Log:    logr.Discard(),
	}

	result := fm.HasFinalizer("invalid-type", SeataFinalizerName)
	if result {
		t.Error("Expected false for unsupported type")
	}
}

func TestFinalizerManager_IsBeingDeleted_UnsupportedType(t *testing.T) {
	fm := &FinalizerManager{
		Client: nil,
		Log:    logr.Discard(),
	}

	result := fm.IsBeingDeleted("invalid-type")
	if result {
		t.Error("Expected false for unsupported type")
	}
}

func createTestScheme() *runtime.Scheme {
	scheme := fake.NewClientBuilder().Build().Scheme()
	_ = seatav1.AddToScheme(scheme)
	return scheme
}
