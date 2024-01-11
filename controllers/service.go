/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
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
