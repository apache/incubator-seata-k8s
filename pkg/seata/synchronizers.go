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
	"time"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/utils"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// HTTP client timeout
	httpClientTimeout = 30 * time.Second
	// HTTP request timeout
	httpRequestTimeout = 10 * time.Second
)

func SyncService(curr *apiv1.Service, next *apiv1.Service) {
	if curr == nil {
		panic("SyncService: current service cannot be nil")
	}
	if next == nil {
		panic("SyncService: next service cannot be nil")
	}
	curr.Spec.Ports = next.Spec.Ports
}

func SyncStatefulSet(curr *appsv1.StatefulSet, next *appsv1.StatefulSet) {
	if curr == nil {
		panic("SyncStatefulSet: current statefulset cannot be nil")
	}
	if next == nil {
		panic("SyncStatefulSet: next statefulset cannot be nil")
	}
	curr.Spec.Template = next.Spec.Template
	curr.Spec.Replicas = next.Spec.Replicas
}

type rspData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
	Success bool   `json:"success"`
}

func changeCluster(ctx context.Context, s *seatav1alpha1.SeataServer, i int32, username string, password string) error {
	if s == nil {
		return fmt.Errorf("changeCluster: SeataServer cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: httpClientTimeout,
	}
	host := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:%d", s.Name, i, s.Spec.ServiceName, s.Namespace, s.Spec.Ports.ConsolePort)

	// Step 1: Login to get token
	values := map[string]string{"username": username, "password": password}
	jsonValue, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to marshal login credentials: %w", err)
	}

	loginURL := fmt.Sprintf("http://%s/api/v1/auth/login", host)
	loginCtx, loginCancel := context.WithTimeout(ctx, httpRequestTimeout)
	defer loginCancel()

	req, err := http.NewRequestWithContext(loginCtx, "POST", loginURL, bytes.NewBuffer(jsonValue))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rsp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send login request to %s: %w", host, err)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status code %d for host %s", rsp.StatusCode, host)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	loginData := &rspData{}
	if err = json.Unmarshal(body, loginData); err != nil {
		return fmt.Errorf("failed to unmarshal login response: %w", err)
	}
	if !loginData.Success {
		return fmt.Errorf("login failed: %s", loginData.Message)
	}
	tokenStr := loginData.Data

	// Step 2: Call changeCluster API
	targetURL := fmt.Sprintf("http://%s/metadata/v1/changeCluster?raftClusterStr=%s",
		host, url.QueryEscape(utils.ConcatRaftServerAddress(s)))

	clusterCtx, clusterCancel := context.WithTimeout(ctx, httpRequestTimeout)
	defer clusterCancel()

	req, err = http.NewRequestWithContext(clusterCtx, "POST", targetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create changeCluster request: %w", err)
	}
	req.Header.Set("Authorization", tokenStr)

	rsp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send changeCluster request to %s: %w", host, err)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("changeCluster failed with status code %d for host %s", rsp.StatusCode, host)
	}

	body, err = io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("failed to read changeCluster response: %w", err)
	}

	clusterData := &rspData{}
	if err = json.Unmarshal(body, clusterData); err != nil {
		return fmt.Errorf("failed to unmarshal changeCluster response: %w", err)
	}

	if !clusterData.Success {
		return fmt.Errorf("changeCluster failed: %s", clusterData.Message)
	}
	return nil
}

func SyncRaftCluster(ctx context.Context, s *seatav1alpha1.SeataServer, username string, password string) error {
	if s == nil {
		return fmt.Errorf("SyncRaftCluster: SeataServer cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	logger := log.FromContext(ctx)
	group, childContext := errgroup.WithContext(ctx)

	for i := int32(0); i < s.Spec.Replicas; i++ {
		finalI := i
		group.Go(func() error {
			// Check if context is already cancelled before proceeding
			select {
			case <-childContext.Done():
				return childContext.Err()
			default:
			}

			err := changeCluster(childContext, s, finalI, username, password)
			if err != nil {
				logger.Error(err, fmt.Sprintf("fail to SyncRaftCluster at %d-th pod", finalI))
			}
			return err
		})
	}
	return group.Wait()
}
