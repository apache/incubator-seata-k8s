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
	"fmt"
	"strconv"
	"strings"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
)

const (
	SeataFinalizer = "cleanUpSeataPVC"
)

// ConcatRaftServerAddress builds a comma-separated list of Raft server addresses
func ConcatRaftServerAddress(s *seatav1alpha1.SeataServer) string {
	var addrBuilder strings.Builder
	for i := int32(0); i < s.Spec.Replicas; i++ {
		// Format: pod-ordinal.service-name:raft-port
		addrBuilder.WriteString(fmt.Sprintf("%s-%d.%s:%d,", s.Name, i, s.Spec.ServiceName, s.Spec.Ports.RaftPort))
	}

	addr := addrBuilder.String()
	if len(addr) > 0 {
		addr = addr[:len(addr)-1] // Remove trailing comma
	}
	return addr
}

// ContainsString checks if a string slice contains a specific string
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// RemoveString removes the first occurrence of a string from a slice
func RemoveString(slice []string, str string) []string {
	var result []string
	for _, item := range slice {
		if item != str {
			result = append(result, item)
		}
	}
	return result
}

// IsPVCOrphan checks if a PVC is orphaned (its ordinal >= replicas)
func IsPVCOrphan(pvcName string, replicas int32) bool {
	// PVC names follow pattern: <name>-<ordinal>
	lastDashIdx := strings.LastIndexAny(pvcName, "-")
	if lastDashIdx == -1 {
		return false
	}

	// Extract and parse the ordinal from PVC name
	ordinal, err := strconv.Atoi(pvcName[lastDashIdx+1:])
	if err != nil {
		return false
	}

	// PVC is orphaned if its ordinal is >= current replica count
	return int32(ordinal) >= replicas
}
