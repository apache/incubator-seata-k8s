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
	"testing"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFetchEnvVar_DirectValue(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name:  "TEST_VAR",
		Value: "test-value",
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}
}

func TestFetchEnvVar_FromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"username": "admin",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-cm",
				},
				Key: "username",
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "admin" {
		t.Errorf("Expected 'admin', got '%s'", result)
	}
}

func TestFetchEnvVar_FromConfigMap_KeyNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"username": "admin",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-cm",
				},
				Key: "nonexistent",
			},
		},
	}

	_, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}
}

func TestFetchEnvVar_FromConfigMap_Optional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"username": "admin",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	optional := true
	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-cm",
				},
				Key:      "nonexistent",
				Optional: &optional,
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string for optional missing key, got '%s'", result)
	}
}

func TestFetchEnvVar_FromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret123"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-secret",
				},
				Key: "password",
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "secret123" {
		t.Errorf("Expected 'secret123', got '%s'", result)
	}
}

func TestFetchEnvVar_FromSecret_KeyNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret123"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-secret",
				},
				Key: "nonexistent",
			},
		},
	}

	_, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}
}

func TestFetchEnvVar_FromSecret_Optional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret123"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	optional := true
	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-secret",
				},
				Key:      "nonexistent",
				Optional: &optional,
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string for optional missing key, got '%s'", result)
	}
}

func TestFetchEnvVar_ConfigMapNotFound_Optional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	optional := true
	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "nonexistent-cm",
				},
				Key:      "key",
				Optional: &optional,
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string for optional missing ConfigMap, got '%s'", result)
	}
}

func TestFetchEnvVar_SecretNotFound_Optional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	optional := true
	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "nonexistent-secret",
				},
				Key:      "key",
				Optional: &optional,
			},
		},
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string for optional missing Secret, got '%s'", result)
	}
}

func TestFetchEnvVar_ConfigMapNotFound_NotOptional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "nonexistent-cm",
				},
				Key: "key",
			},
		},
	}

	_, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err == nil {
		t.Error("Expected error for non-optional missing ConfigMap")
	}
}

func TestFetchEnvVar_SecretNotFound_NotOptional(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name: "TEST_VAR",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "nonexistent-secret",
				},
				Key: "key",
			},
		},
	}

	_, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err == nil {
		t.Error("Expected error for non-optional missing Secret")
	}
}

func TestFetchEnvVar_EmptyValue(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name:  "TEST_VAR",
		Value: "",
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestFetchEnvVar_NoValueFrom(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = seatav1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cr := &seatav1alpha1.SeataServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seata",
			Namespace: "default",
		},
	}

	envVar := v1.EnvVar{
		Name:      "TEST_VAR",
		Value:     "direct-value",
		ValueFrom: nil,
	}

	result, err := FetchEnvVar(context.Background(), fakeClient, cr, envVar)
	if err != nil {
		t.Errorf("FetchEnvVar failed: %v", err)
	}

	if result != "direct-value" {
		t.Errorf("Expected 'direct-value', got '%s'", result)
	}
}
