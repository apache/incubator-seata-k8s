package utils

import (
	"fmt"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"strings"
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
