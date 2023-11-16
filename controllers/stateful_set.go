/*
Copyright 1999-2019 Seata.io Group.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	seatav1alpha1 "github.com/seata/seata-k8s/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

func initStatefulSet(s *seatav1alpha1.SeataServer) *appsv1.StatefulSet {
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
	updateStatefulSet(statefulSet, s)
	return statefulSet
}

func updateStatefulSet(statefulSet *appsv1.StatefulSet, s *seatav1alpha1.SeataServer) {
	var envs []apiv1.EnvVar
	// Create basic environments
	envs = []apiv1.EnvVar{
		{
			Name: "HOST_NAME",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{Name: "store.mode", Value: "raft"},
	}

	statefulSet.Spec.Template.Spec.Containers = []apiv1.Container{{
		Name:  s.Spec.ContainerName,
		Image: s.Spec.Image,
		Command: []string{
			"/bin/bash",
		},
		Args: []string{
			"-c",
			fmt.Sprintf(
				"export SEATA_IP=$(HOST_NAME).%s; /bin/bash /seata-server-entrypoint.sh;",
				s.Spec.ServiceName,
			),
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

	envs = replaceOrAppendEnv(envs, "server.port", strconv.Itoa(int(s.Spec.Ports.ConsolePort)))
	envs = replaceOrAppendEnv(envs, "server.servicePort", strconv.Itoa(int(s.Spec.Ports.ServicePort)))
	var addrBuilder strings.Builder
	for i := int32(0); i < s.Spec.Replicas; i++ {
		// Add governed service name to communicate to each other
		addrBuilder.WriteString(fmt.Sprintf("%s-%d.%s:%d,", s.Name, i, s.Spec.ServiceName, s.Spec.Ports.RaftPort))
		//addrBuilder.WriteString(fmt.Sprintf("%s-%d:%d,", s.Name, i, s.Spec.Ports.RaftPort))
	}
	addr := addrBuilder.String()
	envs = replaceOrAppendEnv(envs, "server.raft.serverAddr", addr[:len(addr)-1])

	for k, v := range s.Spec.Env {
		envs = replaceOrAppendEnv(envs, k, v)
	}
	container.Env = envs
}

func replaceOrAppendEnv(envs []apiv1.EnvVar, key string, value string) []apiv1.EnvVar {
	i := 0
	for ; i < len(envs); i++ {
		if envs[i].Name == key {
			break
		}
	}

	if i == len(envs) {
		// Not found
		envs = append(envs, apiv1.EnvVar{Name: key, Value: value})
	} else {
		envs[i].Value = value
	}
	return envs
}
