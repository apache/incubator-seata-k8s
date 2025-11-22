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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/seata"
	"github.com/apache/seata-k8s/pkg/utils"
)

// SeataServerReconciler reconciles a SeataServer object
type SeataServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

type reconcileFun func(ctx context.Context, s *seatav1alpha1.SeataServer) error

const (
	RequeueSeconds        = 10
	MaxRecentErrorRecords = 5
)

// reconcileClientObject is a generic method to create, update and sync Kubernetes objects
func (r *SeataServerReconciler) reconcileClientObject(
	ctx context.Context,
	s *seatav1alpha1.SeataServer,
	obj client.Object,
	getFunc func() client.Object,
	syncFunc func(found, desired client.Object),
	errorType seatav1alpha1.ServerErrorType,
	objDesc string,
) error {
	// Set controller reference
	if err := controllerutil.SetControllerReference(s, obj, r.Scheme); err != nil {
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			errorType, fmt.Sprintf("Failed to set owner reference for %s", objDesc), err)
		return err
	}

	// Get existing object
	found := getFunc()
	err := r.Client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)

	switch {
	case err != nil && errors.IsNotFound(err):
		// Create new object if not found
		r.Log.Info(fmt.Sprintf("Creating new %s: %s/%s", objDesc, obj.GetNamespace(), obj.GetName()))
		if err := r.Client.Create(ctx, obj); err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				errorType, fmt.Sprintf("Failed to create %s", objDesc), err)
			return err
		}
	case err != nil:
		// Error occurred during get operation
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			errorType, fmt.Sprintf("Failed to get %s", objDesc), err)
		return err
	default:
		// Update existing object
		r.Log.Info(fmt.Sprintf("Updating %s: %s/%s", objDesc, found.GetNamespace(), found.GetName()))
		syncFunc(found, obj)
		if err := r.Client.Update(ctx, found); err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				errorType, fmt.Sprintf("Failed to update %s", objDesc), err)
			return err
		}
	}
	return nil
}

//+kubebuilder:rbac:groups=operator.seata.apache.org,resources=seataservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.seata.apache.org,resources=seataservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.seata.apache.org,resources=seataservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the SeataServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SeataServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	s := &seatav1alpha1.SeataServer{}
	if err := r.Get(ctx, req.NamespacedName, s); err != nil {
		if errors.IsNotFound(err) {
			// Resource already deleted
			return ctrl.Result{}, nil
		}
		r.recordError(ctx, req.NamespacedName, seatav1alpha1.ErrorTypeK8s_SeataServer, "Failed to fetch SeataServer", err)
		return ctrl.Result{}, err
	}

	// Apply default values if needed
	if changed := s.WithDefaults(); changed {
		r.Log.Info("Setting default values for SeataServer", "name", s.Name, "namespace", s.Namespace)
		if err := r.Client.Update(ctx, s); err != nil {
			r.recordError(ctx, req.NamespacedName, seatav1alpha1.ErrorTypeK8s_SeataServer, "Failed to update SeataServer with defaults", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	for _, fun := range []reconcileFun{
		r.reconcileHeadlessService,
		r.reconcileStatefulSet,
		r.reconcileFinalizers,
	} {
		if err := fun(ctx, s); err != nil {
			return reconcile.Result{}, err
		}
	}

	if !s.Status.Synchronized {
		r.Log.Info("SeataServer not synchronized yet, requeuing", "name", s.Name, "namespace", s.Namespace, "requeueAfter", RequeueSeconds)
		return ctrl.Result{Requeue: true, RequeueAfter: RequeueSeconds * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *SeataServerReconciler) reconcileHeadlessService(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	svc := seata.MakeHeadlessService(s)
	return r.reconcileClientObject(
		ctx,
		s,
		svc,
		func() client.Object { return &apiv1.Service{} },
		func(found, desired client.Object) {
			seata.SyncService(found.(*apiv1.Service), desired.(*apiv1.Service))
		},
		seatav1alpha1.ErrorTypeK8s_HeadlessService,
		"Headless Service",
	)
}

func (r *SeataServerReconciler) reconcileStatefulSet(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	sts := seata.MakeStatefulSet(s)
	if err := controllerutil.SetControllerReference(s, sts, r.Scheme); err != nil {
		return err
	}

	foundSts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, foundSts)

	switch {
	case err != nil && errors.IsNotFound(err):
		r.Log.Info(fmt.Sprintf("Creating new StatefulSet: %s/%s", sts.Namespace, sts.Name))
		if err := r.Client.Create(ctx, sts); err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_StatefulSet, "Failed to create StatefulSet", err)
			return err
		}
	case err != nil:
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			seatav1alpha1.ErrorTypeK8s_StatefulSet, "Failed to get StatefulSet", err)
		return err
	default:
		if err := r.updateStatefulSet(ctx, s, foundSts, sts); err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_StatefulSet, "Failed to update StatefulSet", err)
			return err
		}
	}
	return nil
}

func (r *SeataServerReconciler) updateStatefulSet(ctx context.Context, s *seatav1alpha1.SeataServer,
	foundSts *appsv1.StatefulSet, sts *appsv1.StatefulSet) (err error) {

	r.Log.Info(fmt.Sprintf("Updating existing SeataServer StatefulSet {%s:%s}", foundSts.Namespace, foundSts.Name))
	seata.SyncStatefulSet(foundSts, sts)

	err = r.Client.Update(ctx, foundSts)
	if err != nil {
		return err
	}
	s.Status.Replicas = foundSts.Status.Replicas
	s.Status.ReadyReplicas = foundSts.Status.ReadyReplicas

	readySize := foundSts.Status.ReadyReplicas
	newSize := *sts.Spec.Replicas
	if readySize != newSize {
		s.Status.Synchronized = false
	}
	if readySize == newSize && !s.Status.Synchronized {
		username, password := "seata", "seata"
		for _, env := range s.Spec.Env {
			if env.Name == "console.user.username" {
				username, err = seata.FetchEnvVar(ctx, r.Client, s, env)
				if err != nil {
					r.Log.Error(err, "Failed to fetch Env console.user.username")
				}
			}
			if env.Name == "console.user.password" {
				password, err = seata.FetchEnvVar(ctx, r.Client, s, env)
				if err != nil {
					r.Log.Error(err, "Failed to fetch Env console.user.password")
				}
			}
		}
		if err = seata.SyncRaftCluster(ctx, s, username, password); err != nil {
			r.Log.Error(err, "Failed to synchronize the raft cluster")
			s.Status.Synchronized = false
		} else {
			r.Log.Info("Successfully synchronized the raft cluster")
			s.Status.Synchronized = true
		}
	}
	return r.Client.Status().Update(ctx, s)
}

func (r *SeataServerReconciler) reconcileFinalizers(ctx context.Context, instance *seatav1alpha1.SeataServer) (err error) {
	if instance.Spec.Persistence.VolumeReclaimPolicy != seatav1alpha1.VolumeReclaimPolicyDelete {
		return nil
	}
	if instance.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(instance.ObjectMeta.Finalizers, utils.SeataFinalizer) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, utils.SeataFinalizer)
			if err = r.Client.Update(ctx, instance); err != nil {
				r.recordError(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
					seatav1alpha1.ErrorTypeK8s_SeataServer, fmt.Sprintf("Failed to update resource SeataServer(%v) Finalizers", instance.Name), err)
				return err
			}
		}
		return r.cleanupOrphanPVCs(ctx, instance)
	} else {
		if utils.ContainsString(instance.ObjectMeta.Finalizers, utils.SeataFinalizer) {
			if err = r.cleanUpAllPVCs(ctx, instance); err != nil {
				r.recordError(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
					seatav1alpha1.ErrorTypeK8s_SeataServer, fmt.Sprintf("Failed to delete resource SeataServer(%v) PVCs", instance.Name), err)
				return err
			}
			instance.ObjectMeta.Finalizers = utils.RemoveString(instance.ObjectMeta.Finalizers, utils.SeataFinalizer)
			if err = r.Client.Update(ctx, instance); err != nil {
				r.recordError(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace},
					seatav1alpha1.ErrorTypeK8s_SeataServer, fmt.Sprintf("Failed to update resource SeataServer(%v) Finalizers", instance.Name), err)
				return err
			}
		}
	}
	return nil
}

func (r *SeataServerReconciler) cleanupOrphanPVCs(ctx context.Context, s *seatav1alpha1.SeataServer) (err error) {
	// this check should make sure we do not delete the PVCs before the STS has scaled down
	if s.Status.ReadyReplicas == s.Spec.Replicas {
		pvcCount, err := r.getPVCCount(ctx, s)
		if err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("cleanupOrphanPVCs with PVC count %d and ReadyReplicas count %d", pvcCount, s.Status.ReadyReplicas))
		if pvcCount > int(s.Spec.Replicas) {
			pvcList, err := r.getPVCList(ctx, s)
			if err != nil {
				return err
			}
			for _, pvcItem := range pvcList.Items {
				// delete only Orphan PVCs
				if utils.IsPVCOrphan(pvcItem.Name, s.Spec.Replicas) {
					r.deletePVC(ctx, pvcItem)
				}
			}
		}
	}
	return nil
}
func (r *SeataServerReconciler) getPVCCount(ctx context.Context, s *seatav1alpha1.SeataServer) (pvcCount int, err error) {
	pvcList, err := r.getPVCList(ctx, s)
	if err != nil {
		return -1, err
	}
	pvcCount = len(pvcList.Items)
	return pvcCount, nil
}

func (r *SeataServerReconciler) getPVCList(ctx context.Context, s *seatav1alpha1.SeataServer) (pvList apiv1.PersistentVolumeClaimList, err error) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{"app": s.GetName(), "uid": string(s.UID)},
	})
	if err != nil {
		return pvList, err
	}
	pvclistOps := &client.ListOptions{
		Namespace:     s.Namespace,
		LabelSelector: selector,
	}
	pvcList := &apiv1.PersistentVolumeClaimList{}
	err = r.Client.List(ctx, pvcList, pvclistOps)
	return *pvcList, err
}

func (r *SeataServerReconciler) cleanUpAllPVCs(ctx context.Context, s *seatav1alpha1.SeataServer) (err error) {
	pvcList, err := r.getPVCList(ctx, s)
	if err != nil {
		return err
	}
	for _, pvcItem := range pvcList.Items {
		r.deletePVC(ctx, pvcItem)
	}
	return nil
}

func (r *SeataServerReconciler) deletePVC(ctx context.Context, pvcItem apiv1.PersistentVolumeClaim) {
	pvcDelete := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcItem.Name,
			Namespace: pvcItem.Namespace,
		},
	}
	r.Log.Info("Deleting PVC", "name", pvcItem.Name, "namespace", pvcItem.Namespace)
	if err := r.Client.Delete(ctx, pvcDelete); err != nil {
		r.Log.Error(err, "Failed to delete PVC", "name", pvcItem.Name, "namespace", pvcItem.Namespace)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SeataServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&seatav1alpha1.SeataServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&apiv1.Service{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// update SeataServer error status
func (r *SeataServerReconciler) recordError(ctx context.Context, prKey client.ObjectKey, errorType seatav1alpha1.ServerErrorType, errMsg string, err error) error {
	r.Log.Error(err, errMsg)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newError := seatav1alpha1.SeataServerError{
			Type:      errorType.String(),
			Message:   errMsg,
			Timestamp: metav1.Now(),
		}
		toUpdate := seatav1alpha1.SeataServer{}
		err := r.Get(ctx, prKey, &toUpdate)
		if err != nil {
			r.Log.Error(err, "get seata server object error ", "prkey", prKey)
			return err
		}
		// save recently `MaxErrorRecords` error
		if len(toUpdate.Status.Errors) >= MaxRecentErrorRecords {
			toUpdate.Status.Errors = toUpdate.Status.Errors[1:]
		}
		toUpdate.Status.Errors = append(toUpdate.Status.Errors, newError)
		return r.Status().Update(ctx, &toUpdate)
	})
}
