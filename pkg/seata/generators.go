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

package seata

import (
	"fmt"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

func makeLabels(name string) map[string]string {
	return map[string]string{"cr_name": name}
}

func MakeHeadlessService(s *seatav1alpha1.SeataServer) *apiv1.Service {
	labels := makeLabels(s.Name)

	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Spec.ServiceName,
			Namespace: s.Namespace,
			Labels:    labels,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Name: "service-port", Port: s.Spec.Ports.ServicePort},
				{Name: "console-port", Port: s.Spec.Ports.ConsolePort},
				{Name: "raft-port", Port: s.Spec.Ports.RaftPort},
			},
			ClusterIP: "None",
			Selector:  labels,
		},
	}
}

const PythonScript = `
import time
import socket

print('Waiting $SEATA_IP to be resolved...')
while True:
	try:
		ip = socket.gethostbyname('$SEATA_IP')
		print('Resolve $SEATA_IP to', ip)
	except:
		print('Cannot resolve $SEATA_IP, wait 2 seconds', flush=True)
		time.sleep(2)
		continue
	exit(0)
`

func MakeStatefulSet(s *seatav1alpha1.SeataServer) *appsv1.StatefulSet {
	labels := makeLabels(s.Name)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			ServiceName: s.Spec.ServiceName,
			Template:    apiv1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "seata-store"},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{"ReadWriteOnce"},
					Resources:   s.Spec.Store.Resources,
				},
			}},
		},
	}

	statefulSet.Spec.Template.Spec.Containers = []apiv1.Container{{
		Name:  s.Spec.ContainerName,
		Image: s.Spec.Image,
		Command: []string{
			"/bin/bash",
		},
		Args: []string{
			"-c",
			fmt.Sprintf("export SEATA_IP=$(HOST_NAME).%s;", s.Spec.ServiceName) +
				fmt.Sprintf("python3 -c \"\n%s\n\";", PythonScript) +
				"/bin/bash /seata-server-entrypoint.sh;",
		},
		Ports: []apiv1.ContainerPort{
			{Name: "service-port", ContainerPort: s.Spec.Ports.ServicePort},
			{Name: "console-port", ContainerPort: s.Spec.Ports.ConsolePort},
			{Name: "raft-port", ContainerPort: s.Spec.Ports.RaftPort},
		},
		VolumeMounts: []apiv1.VolumeMount{{
			Name:      "seata-store",
			MountPath: "/seata-server/sessionStore",
		}},
	}}

	container := &statefulSet.Spec.Template.Spec.Containers[0]
	container.Resources = s.Spec.Resources
	statefulSet.Spec.Replicas = &s.Spec.Replicas
	container.Image = s.Spec.Image

	container.Ports[0].ContainerPort = s.Spec.Ports.ConsolePort
	container.Ports[1].ContainerPort = s.Spec.Ports.ServicePort
	container.Ports[2].ContainerPort = s.Spec.Ports.RaftPort

	envs := []apiv1.EnvVar{
		{
			Name: "HOST_NAME",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{Name: "store.mode", Value: "raft"},
		{Name: "server.port", Value: strconv.Itoa(int(s.Spec.Ports.ConsolePort))},
		{Name: "server.servicePort", Value: strconv.Itoa(int(s.Spec.Ports.ServicePort))},
	}

	addr := utils.ConcatRaftServerAddress(s)
	envs = append(envs, apiv1.EnvVar{Name: "server.raft.serverAddr", Value: addr})
	for _, env := range s.Spec.Env {
		envs = append(envs, env)
	}
	container.Env = envs

	return statefulSet
}
