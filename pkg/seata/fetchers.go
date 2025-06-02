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
	"context"
	"fmt"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FetchEnvVar(ctx context.Context, c client.Client, cr *seatav1alpha1.SeataServer, envVar v1.EnvVar) (string, error) {
	if envVar.ValueFrom == nil {
		return envVar.Value, nil
	}

	// Inspired by kubelet#makeEnvironmentVariables, determine the final values of variables.
	// See https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kubelet_pods.go#L694-L806
	var result string
	switch {
	case envVar.ValueFrom.ConfigMapKeyRef != nil:
		cm := envVar.ValueFrom.ConfigMapKeyRef
		name := cm.Name
		key := cm.Key
		optional := cm.Optional != nil && *cm.Optional

		configMap := &v1.ConfigMap{}
		err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: cr.Namespace}, configMap)
		if err != nil {
			if errors.IsNotFound(err) && optional {
				// ignore error when marked optional
				return result, nil
			}
			return result, err
		}
		runtimeVal, ok := configMap.Data[key]
		if !ok {
			if optional {
				return result, nil
			}
			return result, fmt.Errorf("couldn't find key %v in ConfigMap %v/%v", key, cr.Namespace, name)
		}
		result = runtimeVal
	case envVar.ValueFrom.SecretKeyRef != nil:
		s := envVar.ValueFrom.SecretKeyRef
		name := s.Name
		key := s.Key
		optional := s.Optional != nil && *s.Optional
		secret := &v1.Secret{}
		err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: cr.Namespace}, secret)
		if err != nil {
			if errors.IsNotFound(err) && optional {
				// ignore error when marked optional
				return result, nil
			}
			return result, err
		}
		runtimeValBytes, ok := secret.Data[key]
		if !ok {
			if optional {
				return result, nil
			}
			return result, fmt.Errorf("couldn't find key %v in Secret %v/%v", key, cr.Namespace, name)
		}
		runtimeVal := string(runtimeValBytes)
		result = runtimeVal
	}
	return result, nil
}
