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

package utils

import (
	"strings"
	"testing"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConcatRaftServerAddress(t *testing.T) {
	testCases := []struct {
		name           string
		seataServer    *seatav1alpha1.SeataServer
		expectedParts  []string
		expectedLength int
	}{
		{
			name: "single replica",
			seataServer: &seatav1alpha1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seata",
				},
				Spec: seatav1alpha1.SeataServerSpec{
					Replicas:    1,
					ServiceName: "seata-svc",
					Ports: seatav1alpha1.Ports{
						RaftPort: 9091,
					},
				},
			},
			expectedParts:  []string{"seata-0.seata-svc:9091"},
			expectedLength: 1,
		},
		{
			name: "three replicas",
			seataServer: &seatav1alpha1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seata-cluster",
				},
				Spec: seatav1alpha1.SeataServerSpec{
					Replicas:    3,
					ServiceName: "seata-headless",
					Ports: seatav1alpha1.Ports{
						RaftPort: 9091,
					},
				},
			},
			expectedParts: []string{
				"seata-cluster-0.seata-headless:9091",
				"seata-cluster-1.seata-headless:9091",
				"seata-cluster-2.seata-headless:9091",
			},
			expectedLength: 3,
		},
		{
			name: "zero replicas",
			seataServer: &seatav1alpha1.SeataServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seata",
				},
				Spec: seatav1alpha1.SeataServerSpec{
					Replicas:    0,
					ServiceName: "seata-svc",
					Ports: seatav1alpha1.Ports{
						RaftPort: 9091,
					},
				},
			},
			expectedParts:  []string{},
			expectedLength: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConcatRaftServerAddress(tc.seataServer)

			if tc.expectedLength == 0 {
				if result != "" {
					t.Errorf("Expected empty string for zero replicas, got '%s'", result)
				}
				return
			}

			// Check that result contains all expected parts
			for _, part := range tc.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("Expected result to contain '%s', but got '%s'", part, result)
				}
			}

			// Check number of addresses (separated by commas)
			addresses := strings.Split(result, ",")
			if len(addresses) != tc.expectedLength {
				t.Errorf("Expected %d addresses, got %d: %v", tc.expectedLength, len(addresses), addresses)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		str      string
		expected bool
	}{
		{
			name:     "string exists in slice",
			slice:    []string{"apple", "banana", "cherry"},
			str:      "banana",
			expected: true,
		},
		{
			name:     "string does not exist in slice",
			slice:    []string{"apple", "banana", "cherry"},
			str:      "orange",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			str:      "test",
			expected: false,
		},
		{
			name:     "empty string in slice",
			slice:    []string{"", "a", "b"},
			str:      "",
			expected: true,
		},
		{
			name:     "single element matching",
			slice:    []string{"only"},
			str:      "only",
			expected: true,
		},
		{
			name:     "single element not matching",
			slice:    []string{"only"},
			str:      "other",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ContainsString(tc.slice, tc.str)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestRemoveString(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		str      string
		expected []string
	}{
		{
			name:     "remove existing string",
			slice:    []string{"apple", "banana", "cherry"},
			str:      "banana",
			expected: []string{"apple", "cherry"},
		},
		{
			name:     "remove non-existing string",
			slice:    []string{"apple", "banana", "cherry"},
			str:      "orange",
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "remove from empty slice",
			slice:    []string{},
			str:      "test",
			expected: []string{},
		},
		{
			name:     "remove only element",
			slice:    []string{"only"},
			str:      "only",
			expected: []string{},
		},
		{
			name:     "remove duplicate strings",
			slice:    []string{"a", "b", "a", "c", "a"},
			str:      "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "remove first element",
			slice:    []string{"first", "second", "third"},
			str:      "first",
			expected: []string{"second", "third"},
		},
		{
			name:     "remove last element",
			slice:    []string{"first", "second", "third"},
			str:      "third",
			expected: []string{"first", "second"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := RemoveString(tc.slice, tc.str)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected length %d, got %d", len(tc.expected), len(result))
				return
			}

			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("At index %d: expected '%s', got '%s'", i, tc.expected[i], v)
				}
			}
		})
	}
}

func TestIsPVCOrphan(t *testing.T) {
	testCases := []struct {
		name     string
		pvcName  string
		replicas int32
		expected bool
	}{
		{
			name:     "PVC within replica range",
			pvcName:  "seata-store-seata-0",
			replicas: 3,
			expected: false,
		},
		{
			name:     "PVC at replica boundary",
			pvcName:  "seata-store-seata-2",
			replicas: 3,
			expected: false,
		},
		{
			name:     "PVC beyond replica range",
			pvcName:  "seata-store-seata-3",
			replicas: 3,
			expected: true,
		},
		{
			name:     "PVC far beyond replica range",
			pvcName:  "seata-store-seata-10",
			replicas: 3,
			expected: true,
		},
		{
			name:     "zero replicas with PVC 0",
			pvcName:  "seata-store-seata-0",
			replicas: 0,
			expected: true,
		},
		{
			name:     "single replica with PVC 1",
			pvcName:  "seata-store-seata-1",
			replicas: 1,
			expected: true,
		},
		{
			name:     "invalid PVC name format",
			pvcName:  "invalid-name-without-number",
			replicas: 3,
			expected: false,
		},
		{
			name:     "PVC name with letters after dash",
			pvcName:  "seata-store-abc",
			replicas: 3,
			expected: false,
		},
		{
			name:     "PVC name without dash",
			pvcName:  "singlename",
			replicas: 3,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPVCOrphan(tc.pvcName, tc.replicas)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for pvcName='%s' with replicas=%d",
					tc.expected, result, tc.pvcName, tc.replicas)
			}
		})
	}
}

func TestSeataFinalizer(t *testing.T) {
	// Test that the constant is defined
	if SeataFinalizer == "" {
		t.Error("SeataFinalizer constant should not be empty")
	}

	if SeataFinalizer != "cleanUpSeataPVC" {
		t.Errorf("Expected SeataFinalizer to be 'cleanUpSeataPVC', got '%s'", SeataFinalizer)
	}
}

func TestConcatRaftServerAddress_CustomPort(t *testing.T) {
	seataServer := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom",
		},
		Spec: seatav1alpha1.SeataServerSpec{
			Replicas:    2,
			ServiceName: "custom-svc",
			Ports: seatav1alpha1.Ports{
				RaftPort: 7777,
			},
		},
	}

	result := ConcatRaftServerAddress(seataServer)

	expectedParts := []string{
		"custom-0.custom-svc:7777",
		"custom-1.custom-svc:7777",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain '%s', but got '%s'", part, result)
		}
	}

	// Should not contain trailing comma
	if strings.HasSuffix(result, ",") {
		t.Error("Result should not have trailing comma")
	}
}

