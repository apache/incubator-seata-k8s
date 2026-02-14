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

package seata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/utils"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func SyncService(curr *apiv1.Service, next *apiv1.Service) {
	curr.Spec.Ports = next.Spec.Ports
}

func SyncStatefulSet(curr *appsv1.StatefulSet, next *appsv1.StatefulSet) {
	curr.Spec.Template = next.Spec.Template
	curr.Spec.Replicas = next.Spec.Replicas
}

type rspData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
	Success bool   `json:"success"`
}

func changeCluster(s *seatav1alpha1.SeataServer, i int32, username string, password string) error {
	host := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:%d", s.Name, i, s.Spec.ServiceName, s.Namespace, s.Spec.Ports.ConsolePort)
	return changeClusterWithHost(host, s, username, password)
}

// changeClusterWithHost allows testing with custom host (exported for testing)
func changeClusterWithHost(host string, s *seatav1alpha1.SeataServer, username string, password string) error {
	client := http.Client{}

	values := map[string]string{"username": username, "password": password}
	jsonValue, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	loginUrl := fmt.Sprintf("http://%s/api/v1/auth/login", host)
	rsp, err := client.Post(loginUrl, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer rsp.Body.Close()

	d := &rspData{}
	var tokenStr string
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}
	if err = json.Unmarshal(body, &d); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}
	if !d.Success {
		return fmt.Errorf("login failed: %s", d.Message)
	}
	tokenStr = d.Data

	targetUrl := fmt.Sprintf("http://%s/metadata/v1/changeCluster?raftClusterStr=%s",
		host, url.QueryEscape(utils.ConcatRaftServerAddress(s)))
	req, err := http.NewRequest("POST", targetUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create changeCluster request: %w", err)
	}
	req.Header.Set("Authorization", tokenStr)
	rsp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("changeCluster request failed: %w", err)
	}
	defer rsp.Body.Close()

	d = &rspData{}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("changeCluster failed with status %d", rsp.StatusCode)
	}

	body, err = io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("failed to read changeCluster response: %w", err)
	}

	if err = json.Unmarshal(body, &d); err != nil {
		return fmt.Errorf("failed to parse changeCluster response: %w", err)
	}

	if !d.Success {
		return fmt.Errorf("changeCluster failed: %s", d.Message)
	}
	return nil
}

func SyncRaftCluster(ctx context.Context, s *seatav1alpha1.SeataServer, username string, password string) error {
	logger := log.FromContext(ctx)
	group, childContext := errgroup.WithContext(ctx)

	for i := int32(0); i < s.Spec.Replicas; i++ {
		finalI := i
		group.Go(func() error {
			select {
			case <-childContext.Done():
				return nil
			default:
				err := changeCluster(s, finalI, username, password)
				if err != nil {
					logger.Error(err, fmt.Sprintf("fail to SyncRaftCluster at %d-th pod", finalI))
				}
				return err
			}
		})
	}
	return group.Wait()
}
