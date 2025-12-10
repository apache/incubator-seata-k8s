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
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seatav1 "github.com/apache/seata-k8s/api/v1"
)

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
			name: "seataserver being deleted",
			seataServer: &seatav1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-seata",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{},
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
	seataServer := &seatav1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	fm := &FinalizerManager{
		Client: nil,
		Log:    logr.Discard(),
	}

	result := fm.GetNamespacedName(seataServer)
	expected := types.NamespacedName{Name: "test-seata", Namespace: "default"}

	if result != expected {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func createTestScheme() *runtime.Scheme {
	scheme := fake.NewClientBuilder().Build().Scheme()
	_ = seatav1.AddToScheme(scheme)
	return scheme
}
