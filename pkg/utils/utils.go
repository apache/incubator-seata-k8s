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
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"strconv"
	"strings"
)

const (
	SeataFinalizer = "cleanUpSeataPVC"
)

func ConcatRaftServerAddress(s *seatav1alpha1.SeataServer) string {
	var addrBuilder strings.Builder
	for i := int32(0); i < s.Spec.Replicas; i++ {
		// Add governed service name to communicate to each other
		addrBuilder.WriteString(fmt.Sprintf("%s-%d.%s:%d,", s.Name, i, s.Spec.ServiceName, s.Spec.Ports.RaftPort))
		//addrBuilder.WriteString(fmt.Sprintf("%s-%d:%d,", s.Name, i, s.Spec.Ports.RaftPort))
	}
	addr := addrBuilder.String()
	addr = addr[:len(addr)-1]
	return addr
}

func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, str string) (result []string) {
	for _, item := range slice {
		if item == str {
			continue
		}
		result = append(result, item)
	}
	return result
}

func IsPVCOrphan(pvcName string, replicas int32) bool {
	index := strings.LastIndexAny(pvcName, "-")
	if index == -1 {
		return false
	}

	ordinal, err := strconv.Atoi(pvcName[index+1:])
	if err != nil {
		return false
	}

	return int32(ordinal) >= replicas
}
