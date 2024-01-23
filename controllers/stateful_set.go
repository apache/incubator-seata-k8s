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
	"bytes"
	"context"
	"errors"
	"fmt"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/go-logr/logr"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"math"
	"net/http"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"time"
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
	return statefulSet
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

func updateStatefulSet(ctx context.Context, statefulSet *appsv1.StatefulSet, s *seatav1alpha1.SeataServer) {
	logger := log.FromContext(ctx)
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

	envs = replaceOrAppendEnv(envs, "server.port", strconv.Itoa(int(s.Spec.Ports.ConsolePort)))
	envs = replaceOrAppendEnv(envs, "server.servicePort", strconv.Itoa(int(s.Spec.Ports.ServicePort)))
	var addrBuilder strings.Builder
	for i := int32(0); i < s.Spec.Replicas; i++ {
		// Add governed service name to communicate to each other
		addrBuilder.WriteString(fmt.Sprintf("%s-%d.%s:%d,", s.Name, i, s.Spec.ServiceName, s.Spec.Ports.RaftPort))
		//addrBuilder.WriteString(fmt.Sprintf("%s-%d:%d,", s.Name, i, s.Spec.Ports.RaftPort))
	}
	addr := addrBuilder.String()
	addr = addr[:len(addr)-1]

	envs = replaceOrAppendEnv(envs, "server.raft.serverAddr", addr)
	for k, v := range s.Spec.Env {
		envs = replaceOrAppendEnv(envs, k, v)
	}
	container.Env = envs

	go func(s seatav1alpha1.SeataServer) {
		healthyCheck(logger, s)
		if err := changeCluster(s, addr); err != nil {
			logger.Error(err, "error during calling api")
		} else {
			logger.Info("call api to change cluster successfully")
		}
	}(*s)
}

func healthyCheck(logger logr.Logger, s seatav1alpha1.SeataServer) {
	const exponentialBackoffCeilingSecs int64 = 10 * 60
	lastUpdatedAt := time.Now()
	attempts := 0

	var delaySecs int64
	for delaySecs != exponentialBackoffCeilingSecs {
		success := true
		for i := int32(0); i < s.Spec.Replicas; i++ {
			rsp, err := http.Get(fmt.Sprintf("http://%s-%d.%s.%s.svc:%d",
				s.Name, i, s.Spec.ServiceName, s.Namespace, s.Spec.Ports.ConsolePort))
			if err != nil || rsp.StatusCode != 200 {
				logger.Error(err, fmt.Sprintf("error during healthy check, err %v, rsp %v", err, rsp))
				success = false
				break
			}
		}
		if success {
			return
		}

		// Use exponential backoff to check health
		if time.Now().Sub(lastUpdatedAt).Hours() >= 12 {
			attempts = 0
		}
		lastUpdatedAt = time.Now()
		attempts += 1
		delaySecs = int64(math.Floor((math.Pow(2, float64(attempts)) - 1) * 0.5))
		if delaySecs > exponentialBackoffCeilingSecs {
			delaySecs = exponentialBackoffCeilingSecs
		}
		logger.Info(fmt.Sprintf("%d-th attempt for healthy check, waiting %d seconds", attempts, delaySecs))
		time.Sleep(time.Duration(delaySecs) * time.Second)
	}
	logger.Info("attempt for healthy check failed")
}

type rspData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
	Success bool   `json:"success"`
}

func changeCluster(s seatav1alpha1.SeataServer, raftClusterStr string) error {
	client := http.Client{}
	for i := int32(0); i < s.Spec.Replicas; i++ {
		// Add governed service name to communicate to each other
		host := fmt.Sprintf("%s-%d.%s.%s.svc:%d", s.Name, i, s.Spec.ServiceName, s.Namespace, s.Spec.Ports.ConsolePort)
		username, ok := s.Spec.Env["console.user.username"]
		if !ok {
			username = "seata"
		}
		password, ok := s.Spec.Env["console.user.password"]
		if !ok {
			password = "seata"
		}

		values := map[string]string{"username": username, "password": password}
		jsonValue, _ := json.Marshal(values)
		loginUrl := fmt.Sprintf("http://%s/api/v1/auth/login", host)
		rsp, err := client.Post(loginUrl, "application/json", bytes.NewBuffer(jsonValue))
		if err != nil {
			return err
		}

		d := &rspData{}
		var tokenStr string
		if rsp.StatusCode != http.StatusOK {
			return errors.New("login failed")
		}

		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(body, &d); err != nil {
			return err
		}
		if !d.Success {
			return errors.New(d.Message)
		}
		tokenStr = d.Data

		targetUrl := fmt.Sprintf("http://%s/metadata/v1/changeCluster?raftClusterStr=%s",
			host, url.QueryEscape(raftClusterStr))
		req, _ := http.NewRequest("POST", targetUrl, nil)
		req.Header.Set("Authorization", tokenStr)
		rsp, err = client.Do(req)
		if err != nil {
			return err
		}

		d = &rspData{}
		if rsp.StatusCode != http.StatusOK {
			return errors.New("failed to changeCluster")
		}

		body, err = io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(body, &d); err != nil {
			return err
		}

		if !d.Success {
			return errors.New(d.Message)
		}
	}
	return nil
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
