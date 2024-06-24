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
