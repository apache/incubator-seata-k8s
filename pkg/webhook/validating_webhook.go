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

package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	seatav1 "github.com/apache/seata-k8s/api/v1"
)

// ValidatingWebhookHandler handles validation webhook requests
type ValidatingWebhookHandler struct {
	decoder runtime.Decoder
}

// NewValidatingWebhookHandler creates a new ValidatingWebhookHandler
func NewValidatingWebhookHandler(decoder runtime.Decoder) *ValidatingWebhookHandler {
	return &ValidatingWebhookHandler{
		decoder: decoder,
	}
}

// HandleSeataServerValidation handles SeataServer validation webhook requests
func (h *ValidatingWebhookHandler) HandleSeataServerValidation(w http.ResponseWriter, r *http.Request) {
	// Read admission review from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	admissionReview := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal admission review: %v", err), http.StatusBadRequest)
		return
	}

	// Validate the AdmissionRequest
	if admissionReview.Request == nil {
		http.Error(w, "admission request is nil", http.StatusBadRequest)
		return
	}

	// Parse the SeataServer object
	seataServer := &seatav1.SeataServer{}
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, seataServer); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: &admissionv1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: false,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("failed to unmarshal SeataServer: %v", err),
				},
			},
		})
		return
	}

	// Perform validation
	validationErrors := ValidateSeataServer(seataServer)

	// Build admission response
	response := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID: admissionReview.Request.UID,
		},
	}

	if len(validationErrors) == 0 {
		response.Response.Allowed = true
	} else {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusUnprocessableEntity,
			Message: strings.Join(validationErrors, "; "),
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ValidateSeataServer performs validation on SeataServer resource
func ValidateSeataServer(server *seatav1.SeataServer) []string {
	var errors []string

	if server == nil {
		return append(errors, "SeataServer object is nil")
	}

	// Validate service name
	if err := validateServiceName(server.Spec.ServiceName); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate replicas
	if err := validateReplicas(server.Spec.Replicas); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate image
	if err := validateImage(server.Spec.Image); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate ports
	if err := validatePorts(server.Spec.Ports); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate storage configuration
	if err := validateStorage(&server.Spec.Persistence); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate resources
	if err := validateResourceRequirements(server.Spec.Resources); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate volume reclaim policy
	if err := validateVolumeReclaimPolicy(server.Spec.Persistence.VolumeReclaimPolicy); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate environment variables
	if err := validateEnvironmentVariables(server.Spec.Env); err != nil {
		errors = append(errors, err.Error())
	}

	return errors
}

// validateServiceName validates the service name
func validateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("serviceName must not be empty")
	}

	// DNS-1035 label validation
	dnsRegex := regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)
	if !dnsRegex.MatchString(name) {
		return fmt.Errorf("serviceName must be a valid DNS-1035 label (lowercase alphanumeric and hyphens, must start with letter)")
	}

	if len(name) > 63 {
		return fmt.Errorf("serviceName must be no more than 63 characters")
	}

	return nil
}

// validateReplicas validates the number of replicas
func validateReplicas(replicas int32) error {
	if replicas < 1 {
		return fmt.Errorf("replicas must be at least 1")
	}

	if replicas > 1000 {
		return fmt.Errorf("replicas must not exceed 1000")
	}

	return nil
}

// validateImage validates the Docker image reference
func validateImage(image string) error {
	if image == "" {
		return fmt.Errorf("image must not be empty")
	}

	// Basic Docker image reference validation
	// Format: [REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG][@DIGEST]
	imageRegex := regexp.MustCompile(`^([a-z0-9]+(?:[._-][a-z0-9]+)*/)*[a-z0-9]+(?:[._-][a-z0-9]+)*(?::[a-zA-Z0-9_][a-zA-Z0-9._-]*)?(?:@[a-zA-Z0-9:]+)?$`)
	if !imageRegex.MatchString(image) {
		return fmt.Errorf("image must be a valid Docker image reference")
	}

	return nil
}

// validatePorts validates port numbers
func validatePorts(ports seatav1.Ports) error {
	// Validate port numbers
	if ports.ServicePort < 1 || ports.ServicePort > 65535 {
		return fmt.Errorf("servicePort must be between 1 and 65535")
	}

	if ports.ConsolePort < 1 || ports.ConsolePort > 65535 {
		return fmt.Errorf("consolePort must be between 1 and 65535")
	}

	if ports.RaftPort < 1 || ports.RaftPort > 65535 {
		return fmt.Errorf("raftPort must be between 1 and 65535")
	}

	// Check for port conflicts
	portMap := make(map[int32]string)
	portMap[ports.ServicePort] = "servicePort"
	portMap[ports.ConsolePort] = "consolePort"
	portMap[ports.RaftPort] = "raftPort"

	if len(portMap) < 3 {
		return fmt.Errorf("servicePort, consolePort, and raftPort must be different")
	}

	return nil
}

// validateStorage validates storage configuration
func validateStorage(persistence *seatav1.Persistence) error {
	if persistence == nil {
		return nil
	}

	spec := persistence.PersistentVolumeClaimSpec
	if spec.Resources.Requests == nil {
		return fmt.Errorf("storage must be specified in persistence.spec.resources.requests")
	}

	storage, exists := spec.Resources.Requests["storage"]
	if !exists {
		return fmt.Errorf("storage must be specified in persistence.spec.resources.requests")
	}

	// Check storage size range (1Gi to 1000Gi)
	minStorage := resource.MustParse("1Gi")
	maxStorage := resource.MustParse("1000Gi")

	if storage.Cmp(minStorage) < 0 {
		return fmt.Errorf("storage size must be at least 1Gi")
	}

	if storage.Cmp(maxStorage) > 0 {
		return fmt.Errorf("storage size must not exceed 1000Gi")
	}

	return nil
}

// validateResourceRequirements validates CPU and memory resources
func validateResourceRequirements(resources apiv1.ResourceRequirements) error {
	// Validate requests
	for resource, quantity := range resources.Requests {
		if err := validateResourceQuantity(string(resource), quantity); err != nil {
			return err
		}
	}

	// Validate limits
	for resource, quantity := range resources.Limits {
		if err := validateResourceQuantity(string(resource), quantity); err != nil {
			return err
		}
	}

	// Ensure limits are greater than or equal to requests
	cpuRequest := resources.Requests["cpu"]
	cpuLimit := resources.Limits["cpu"]
	if !cpuRequest.IsZero() && !cpuLimit.IsZero() && cpuLimit.Cmp(cpuRequest) < 0 {
		return fmt.Errorf("cpu limit must be greater than or equal to cpu request")
	}

	memRequest := resources.Requests["memory"]
	memLimit := resources.Limits["memory"]
	if !memRequest.IsZero() && !memLimit.IsZero() && memLimit.Cmp(memRequest) < 0 {
		return fmt.Errorf("memory limit must be greater than or equal to memory request")
	}

	return nil
}

// validateResourceQuantity validates a single resource quantity
func validateResourceQuantity(resourceName string, quantity resource.Quantity) error {
	if quantity.IsZero() {
		return nil
	}

	// Check for negative values
	if quantity.Sign() < 0 {
		return fmt.Errorf("%s must be non-negative", resourceName)
	}

	return nil
}

// validateVolumeReclaimPolicy validates the volume reclaim policy
func validateVolumeReclaimPolicy(policy seatav1.VolumeReclaimPolicy) error {
	if policy == "" {
		return nil // Empty is valid, will use default
	}

	if policy != seatav1.VolumeReclaimPolicyRetain && policy != seatav1.VolumeReclaimPolicyDelete {
		return fmt.Errorf("volumeReclaimPolicy must be 'Retain' or 'Delete'")
	}

	return nil
}

// validateEnvironmentVariables validates environment variables
func validateEnvironmentVariables(envVars []apiv1.EnvVar) error {
	validNameRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

	for _, env := range envVars {
		if !validNameRegex.MatchString(env.Name) {
			return fmt.Errorf("environment variable name '%s' is invalid (must start with letter or underscore, contain only alphanumeric and underscores)", env.Name)
		}

		if len(env.Name) > 250 {
			return fmt.Errorf("environment variable name '%s' is too long (max 250 characters)", env.Name)
		}
	}

	return nil
}
