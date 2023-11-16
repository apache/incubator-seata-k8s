package controllers

import (
	seatav1alpha1 "github.com/seata/seata-k8s/api/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func initService(s *seatav1alpha1.SeataServer) *apiv1.Service {
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Spec.ServiceName,
			Namespace: s.Namespace,
			Labels:    makeLabels(s.Name),
		},
	}
	updateService(service, s)
	return service
}

func updateService(service *apiv1.Service, s *seatav1alpha1.SeataServer) {
	service.Spec = apiv1.ServiceSpec{
		Ports: []apiv1.ServicePort{
			{Name: "service-port", Port: s.Spec.Ports.ServicePort},
			{Name: "console-port", Port: s.Spec.Ports.ConsolePort},
			{Name: "raft-port", Port: s.Spec.Ports.RaftPort},
		},
		ClusterIP: "None",
		Selector:  makeLabels(s.Name),
	}
}
