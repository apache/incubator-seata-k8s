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
			r.recordError(ctx, req.NamespacedName, seatav1alpha1.ErrorTypeK8s_SeataServer, fmt.Sprintf("Failed to get resource SeataServer(%v),err %s", req.NamespacedName, err.Error()), err)
			return ctrl.Result{}, nil
		}
	}

	changed := s.WithDefaults()
	if changed {
		r.Log.Info(fmt.Sprintf("Setting default values for SeataServer(%v)", req.NamespacedName))
		if err := r.Client.Update(ctx, s); err != nil {
			r.recordError(ctx, req.NamespacedName, seatav1alpha1.ErrorTypeK8s_SeataServer, fmt.Sprintf("Failed to update resource SeataServer(%v)", req.NamespacedName), err)
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
		r.Log.Info(fmt.Sprintf("SeataServer(%v) has not been synchronized yet, requeue in %d seconds",
			req.NamespacedName, RequeueSeconds))
		return ctrl.Result{Requeue: true, RequeueAfter: RequeueSeconds * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *SeataServerReconciler) reconcileHeadlessService(ctx context.Context, s *seatav1alpha1.SeataServer) (err error) {
	svc := seata.MakeHeadlessService(s)
	if err := controllerutil.SetControllerReference(s, svc, r.Scheme); err != nil {
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			seatav1alpha1.ErrorTypeK8s_HeadlessService, fmt.Sprintf("Failed to set owner reference for SeataServer(%v)", s.Name), err)
		return err
	}
	foundSvc := &apiv1.Service{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info(fmt.Sprintf("Creating a new SeataServer Service {%s:%s}",
			svc.Namespace, svc.Name))
		err = r.Client.Create(ctx, svc)
		if err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_HeadlessService, fmt.Sprintf("Failed to create SeataServer Service {%s:%s}",
					svc.Namespace, svc.Name), err)
			return err
		}
		return nil
	} else if err != nil {
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			seatav1alpha1.ErrorTypeK8s_HeadlessService, fmt.Sprintf("Failed to get SeataServer Service {%s:%s}",
				svc.Namespace, svc.Name), err)
		return err
	} else {
		r.Log.Info(fmt.Sprintf("Updating existing SeataServer Service {%s:%s}",
			foundSvc.Namespace, foundSvc.Name))
		seata.SyncService(foundSvc, svc)
		err = r.Client.Update(ctx, foundSvc)
		if err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_HeadlessService, fmt.Sprintf("Failed to update SeataServer Service {%s:%s}",
					foundSvc.Namespace, foundSvc.Name), err)
			return err
		}
	}
	return nil
}

func (r *SeataServerReconciler) reconcileStatefulSet(ctx context.Context, s *seatav1alpha1.SeataServer) (err error) {
	sts := seata.MakeStatefulSet(s)
	if err := controllerutil.SetControllerReference(s, sts, r.Scheme); err != nil {
		return err
	}
	foundSts := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      sts.Name,
		Namespace: sts.Namespace,
	}, foundSts)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info(fmt.Sprintf("Creating a new SeataServer StatefulSet {%s:%s}",
			sts.Namespace, sts.Name))
		err = r.Client.Create(ctx, sts)
		if err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_StatefulSet, fmt.Sprintf("Failed to create SeataServer StatefulSet {%s:%s}",
					sts.Namespace, sts.Name), err)
			return err
		}
		return nil
	} else if err != nil {
		r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
			seatav1alpha1.ErrorTypeK8s_StatefulSet, fmt.Sprintf("Failed to get SeataServer StatefulSet {%s:%s}",
				sts.Namespace, sts.Name), err)
		return err
	} else {
		err = r.updateStatefulSet(ctx, s, foundSts, sts)
		if err != nil {
			r.recordError(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace},
				seatav1alpha1.ErrorTypeK8s_StatefulSet, fmt.Sprintf("Failed to update SeataServer StatefulSet {%s:%s}",
					foundSts.Namespace, foundSts.Name), err)
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
	r.Log.Info(fmt.Sprintf("Deleting PVC with name %s", pvcItem.Name))
	err := r.Client.Delete(ctx, pvcDelete)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Error deleting PVC with name %s", pvcDelete))
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
