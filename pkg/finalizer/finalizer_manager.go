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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	seatav1 "github.com/apache/seata-k8s/api/v1"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/utils"
)

const (
	// SeataFinalizerName is the name of the finalizer for SeataServer resources
	SeataFinalizerName = "seataserver.finalizer.operator.seata.apache.org/cleanup"

	// DeprecatedSeataFinalizerName is the old finalizer name for backward compatibility
	DeprecatedSeataFinalizerName = "seataserver.finalizer"
)

// FinalizerManager manages the lifecycle of SeataServer resources through finalizers
type FinalizerManager struct {
	Client client.Client
	Log    logr.Logger
}

// NewFinalizerManager creates a new FinalizerManager instance
func NewFinalizerManager(client client.Client, log logr.Logger) *FinalizerManager {
	return &FinalizerManager{
		Client: client,
		Log:    log,
	}
}

// AddFinalizer adds the finalizer to the SeataServer object if not already present
func (fm *FinalizerManager) AddFinalizer(ctx context.Context, server interface{}, finalizerName string) error {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return fm.addFinalizerToV1(ctx, s, finalizerName)
	case *seatav1alpha1.SeataServer:
		return fm.addFinalizerToV1Alpha1(ctx, s, finalizerName)
	default:
		return fmt.Errorf("unsupported type: %T", server)
	}
}

// RemoveFinalizer removes the finalizer from the SeataServer object
func (fm *FinalizerManager) RemoveFinalizer(ctx context.Context, server interface{}, finalizerName string) error {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return fm.removeFinalizerFromV1(ctx, s, finalizerName)
	case *seatav1alpha1.SeataServer:
		return fm.removeFinalizerFromV1Alpha1(ctx, s, finalizerName)
	default:
		return fmt.Errorf("unsupported type: %T", server)
	}
}

// HasFinalizer checks if the SeataServer has the specified finalizer
func (fm *FinalizerManager) HasFinalizer(server interface{}, finalizerName string) bool {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName)
	case *seatav1alpha1.SeataServer:
		return utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName)
	default:
		return false
	}
}

// IsBeingDeleted checks if the SeataServer is marked for deletion
func (fm *FinalizerManager) IsBeingDeleted(server interface{}) bool {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return !s.DeletionTimestamp.IsZero()
	case *seatav1alpha1.SeataServer:
		return !s.DeletionTimestamp.IsZero()
	default:
		return false
	}
}

// GetDeletionTimestamp returns the deletion timestamp of the SeataServer
func (fm *FinalizerManager) GetDeletionTimestamp(server interface{}) interface{} {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return s.DeletionTimestamp
	case *seatav1alpha1.SeataServer:
		return s.DeletionTimestamp
	default:
		return nil
	}
}

// GetNamespacedName returns the NamespacedName of the SeataServer
func (fm *FinalizerManager) GetNamespacedName(server interface{}) types.NamespacedName {
	switch s := server.(type) {
	case *seatav1.SeataServer:
		return types.NamespacedName{Name: s.Name, Namespace: s.Namespace}
	case *seatav1alpha1.SeataServer:
		return types.NamespacedName{Name: s.Name, Namespace: s.Namespace}
	default:
		return types.NamespacedName{}
	}
}

// Helper methods for v1 API

func (fm *FinalizerManager) addFinalizerToV1(ctx context.Context, s *seatav1.SeataServer, finalizerName string) error {
	if utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName) {
		return nil // Already has finalizer
	}

	fm.Log.Info("Adding finalizer", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)

	s.ObjectMeta.Finalizers = append(s.ObjectMeta.Finalizers, finalizerName)
	if err := fm.Client.Update(ctx, s); err != nil {
		fm.Log.Error(err, "Failed to add finalizer", "seataserver", s.Name, "namespace", s.Namespace)
		return err
	}

	fm.Log.Info("Finalizer added successfully", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)
	return nil
}

func (fm *FinalizerManager) removeFinalizerFromV1(ctx context.Context, s *seatav1.SeataServer, finalizerName string) error {
	if !utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName) {
		return nil // Finalizer already removed
	}

	fm.Log.Info("Removing finalizer", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)

	s.ObjectMeta.Finalizers = utils.RemoveString(s.ObjectMeta.Finalizers, finalizerName)
	if err := fm.Client.Update(ctx, s); err != nil {
		fm.Log.Error(err, "Failed to remove finalizer", "seataserver", s.Name, "namespace", s.Namespace)
		return err
	}

	fm.Log.Info("Finalizer removed successfully", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)
	return nil
}

// Helper methods for v1alpha1 API

func (fm *FinalizerManager) addFinalizerToV1Alpha1(ctx context.Context, s *seatav1alpha1.SeataServer, finalizerName string) error {
	if utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName) {
		return nil // Already has finalizer
	}

	fm.Log.Info("Adding finalizer", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)

	s.ObjectMeta.Finalizers = append(s.ObjectMeta.Finalizers, finalizerName)
	if err := fm.Client.Update(ctx, s); err != nil {
		fm.Log.Error(err, "Failed to add finalizer", "seataserver", s.Name, "namespace", s.Namespace)
		return err
	}

	fm.Log.Info("Finalizer added successfully", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)
	return nil
}

func (fm *FinalizerManager) removeFinalizerFromV1Alpha1(ctx context.Context, s *seatav1alpha1.SeataServer, finalizerName string) error {
	if !utils.ContainsString(s.ObjectMeta.Finalizers, finalizerName) {
		return nil // Finalizer already removed
	}

	fm.Log.Info("Removing finalizer", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)

	s.ObjectMeta.Finalizers = utils.RemoveString(s.ObjectMeta.Finalizers, finalizerName)
	if err := fm.Client.Update(ctx, s); err != nil {
		fm.Log.Error(err, "Failed to remove finalizer", "seataserver", s.Name, "namespace", s.Namespace)
		return err
	}

	fm.Log.Info("Finalizer removed successfully", "name", finalizerName, "seataserver", s.Name, "namespace", s.Namespace)
	return nil
}
