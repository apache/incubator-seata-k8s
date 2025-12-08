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

package finalizer

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	seatav1 "github.com/apache/seata-k8s/api/v1"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
)

// CleanupHandler handles cleanup operations when SeataServer is deleted
type CleanupHandler struct {
	Client client.Client
	Log    logr.Logger
}

// NewCleanupHandler creates a new CleanupHandler instance
func NewCleanupHandler(client client.Client, log logr.Logger) *CleanupHandler {
	return &CleanupHandler{
		Client: client,
		Log:    log,
	}
}

// HandleCleanup performs cleanup operations based on the SeataServer configuration
func (ch *CleanupHandler) HandleCleanup(ctx context.Context, server interface{}) error {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return ch.handleCleanupV1(ctx, s)
	case *seatav1alpha1.SeataServer:
		return ch.handleCleanupV1Alpha1(ctx, s)
	default:
		return fmt.Errorf("unsupported type: %T", server)
	}
}

// handleCleanupV1 performs cleanup for v1 SeataServer
func (ch *CleanupHandler) handleCleanupV1(ctx context.Context, s *seatav1.SeataServer) error {
	ch.Log.Info("Starting cleanup for SeataServer", "name", s.Name, "namespace", s.Namespace)

	// Only cleanup PVCs if reclaim policy is Delete
	if s.Spec.Persistence.VolumeReclaimPolicy == seatav1.VolumeReclaimPolicyDelete {
		ch.Log.Info("Cleaning up PVCs for SeataServer", "name", s.Name, "namespace", s.Namespace)
		if err := ch.cleanupAllPVCs(ctx, s.Name, s.Namespace, s.UID); err != nil {
			ch.Log.Error(err, "Failed to cleanup PVCs", "seataserver", s.Name, "namespace", s.Namespace)
			return err
		}
	}

	// Cleanup other resources as needed
	if err := ch.cleanupConfigMaps(ctx, s.Name, s.Namespace); err != nil {
		ch.Log.Error(err, "Failed to cleanup ConfigMaps", "seataserver", s.Name, "namespace", s.Namespace)
		// Continue cleanup despite error
	}

	if err := ch.cleanupSecrets(ctx, s.Name, s.Namespace); err != nil {
		ch.Log.Error(err, "Failed to cleanup Secrets", "seataserver", s.Name, "namespace", s.Namespace)
		// Continue cleanup despite error
	}

	ch.Log.Info("Cleanup completed for SeataServer", "name", s.Name, "namespace", s.Namespace)
	return nil
}

// handleCleanupV1Alpha1 performs cleanup for v1alpha1 SeataServer
func (ch *CleanupHandler) handleCleanupV1Alpha1(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	ch.Log.Info("Starting cleanup for SeataServer (v1alpha1)", "name", s.Name, "namespace", s.Namespace)

	// Only cleanup PVCs if reclaim policy is Delete
	if s.Spec.Persistence.VolumeReclaimPolicy == seatav1alpha1.VolumeReclaimPolicyDelete {
		ch.Log.Info("Cleaning up PVCs for SeataServer (v1alpha1)", "name", s.Name, "namespace", s.Namespace)
		if err := ch.cleanupAllPVCs(ctx, s.Name, s.Namespace, s.UID); err != nil {
			ch.Log.Error(err, "Failed to cleanup PVCs", "seataserver", s.Name, "namespace", s.Namespace)
			return err
		}
	}

	// Cleanup other resources as needed
	if err := ch.cleanupConfigMaps(ctx, s.Name, s.Namespace); err != nil {
		ch.Log.Error(err, "Failed to cleanup ConfigMaps", "seataserver", s.Name, "namespace", s.Namespace)
		// Continue cleanup despite error
	}

	if err := ch.cleanupSecrets(ctx, s.Name, s.Namespace); err != nil {
		ch.Log.Error(err, "Failed to cleanup Secrets", "seataserver", s.Name, "namespace", s.Namespace)
		// Continue cleanup despite error
	}

	ch.Log.Info("Cleanup completed for SeataServer (v1alpha1)", "name", s.Name, "namespace", s.Namespace)
	return nil
}

// cleanupAllPVCs deletes all PVCs associated with the SeataServer
func (ch *CleanupHandler) cleanupAllPVCs(ctx context.Context, name, namespace string, uid interface{}) error {
	pvcList, err := ch.getPVCList(ctx, name, namespace, uid)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // PVCs not found, no cleanup needed
		}
		return err
	}

	ch.Log.Info("Found PVCs to cleanup", "count", len(pvcList.Items), "seataserver", name, "namespace", namespace)

	for _, pvc := range pvcList.Items {
		if err := ch.deletePVC(ctx, &pvc); err != nil {
			ch.Log.Error(err, "Failed to delete PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
			// Continue deleting other PVCs despite error
		}
	}

	return nil
}

// getPVCList retrieves all PVCs associated with the SeataServer
func (ch *CleanupHandler) getPVCList(ctx context.Context, name, namespace string, uid interface{}) (*apiv1.PersistentVolumeClaimList, error) {
	selector := labels.SelectorFromSet(map[string]string{
		"app": name,
		"uid": fmt.Sprintf("%v", uid),
	})

	pvcList := &apiv1.PersistentVolumeClaimList{}
	listOpts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	}

	if err := ch.Client.List(ctx, pvcList, listOpts); err != nil {
		ch.Log.Error(err, "Failed to list PVCs", "seataserver", name, "namespace", namespace)
		return nil, err
	}

	return pvcList, nil
}

// deletePVC deletes a single PVC
func (ch *CleanupHandler) deletePVC(ctx context.Context, pvc *apiv1.PersistentVolumeClaim) error {
	ch.Log.Info("Deleting PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)

	if err := ch.Client.Delete(ctx, pvc); err != nil {
		if !errors.IsNotFound(err) {
			ch.Log.Error(err, "Failed to delete PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
			return err
		}
	}

	return nil
}

// cleanupConfigMaps deletes ConfigMaps associated with the SeataServer
func (ch *CleanupHandler) cleanupConfigMaps(ctx context.Context, name, namespace string) error {
	cmList := &apiv1.ConfigMapList{}
	selector := labels.SelectorFromSet(map[string]string{"app": name})
	listOpts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	}

	if err := ch.Client.List(ctx, cmList, listOpts); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	for _, cm := range cmList.Items {
		ch.Log.Info("Deleting ConfigMap", "configmap", cm.Name, "namespace", cm.Namespace)
		if err := ch.Client.Delete(ctx, &cm); err != nil {
			if !errors.IsNotFound(err) {
				ch.Log.Error(err, "Failed to delete ConfigMap", "configmap", cm.Name, "namespace", cm.Namespace)
				// Continue cleanup despite error
			}
		}
	}

	return nil
}

// cleanupSecrets deletes Secrets associated with the SeataServer
func (ch *CleanupHandler) cleanupSecrets(ctx context.Context, name, namespace string) error {
	secretList := &apiv1.SecretList{}
	selector := labels.SelectorFromSet(map[string]string{"app": name})
	listOpts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	}

	if err := ch.Client.List(ctx, secretList, listOpts); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	for _, secret := range secretList.Items {
		ch.Log.Info("Deleting Secret", "secret", secret.Name, "namespace", secret.Namespace)
		if err := ch.Client.Delete(ctx, &secret); err != nil {
			if !errors.IsNotFound(err) {
				ch.Log.Error(err, "Failed to delete Secret", "secret", secret.Name, "namespace", secret.Namespace)
				// Continue cleanup despite error
			}
		}
	}

	return nil
}
