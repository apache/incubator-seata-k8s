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
	"context"
	"fmt"
	"time"

	"github.com/apache/seata-k8s/pkg/seata"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
)

// SeataServerReconciler reconciles a SeataServer object
type SeataServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type reconcileFun func(ctx context.Context, s *seatav1alpha1.SeataServer) error

const RequeueSeconds = 10

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
	logger := log.FromContext(ctx)

	s := &seatav1alpha1.SeataServer{}
	if err := r.Get(ctx, req.NamespacedName, s); err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("SeataServer(%v) resource not found", req.NamespacedName))
			return ctrl.Result{}, nil
		}

		logger.Error(err, fmt.Sprintf("Failed to get resource SeataServer(%v)", req.NamespacedName))
		return ctrl.Result{}, err
	}
	setupDefaults(s)

	for _, fun := range []reconcileFun{
		r.reconcileHeadlessService,
		r.reconcileStatefulSet,
	} {
		if err := fun(ctx, s); err != nil {
			return reconcile.Result{}, err
		}
	}

	if !s.Status.Synchronized {
		logger.Info(fmt.Sprintf("SeataServer(%v) has not been synchronized yet, requeue in %d seconds",
			req.NamespacedName, RequeueSeconds))
		return ctrl.Result{Requeue: true, RequeueAfter: RequeueSeconds * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *SeataServerReconciler) reconcileHeadlessService(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	logger := log.FromContext(ctx)

	svc := seata.MakeHeadlessService(s)
	if err := controllerutil.SetControllerReference(s, svc, r.Scheme); err != nil {
		return err
	}
	foundSvc := &apiv1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, foundSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Creating a new SeataServer Service {%s:%s}",
				svc.Namespace, svc.Name))
			return r.Client.Create(ctx, svc)
		}
		return err
	}

	logger.Info(fmt.Sprintf("Updating existing SeataServer StatefulSet {%s:%s}",
		foundSvc.Namespace, foundSvc.Name))
	seata.SyncService(foundSvc, svc)
	return r.Client.Update(ctx, foundSvc)
}

func (r *SeataServerReconciler) reconcileStatefulSet(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	logger := log.FromContext(ctx)

	sts := seata.MakeStatefulSet(s)
	if err := controllerutil.SetControllerReference(s, sts, r.Scheme); err != nil {
		return err
	}
	foundSts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      sts.Name,
		Namespace: sts.Namespace,
	}, foundSts)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Creating a new SeataServer StatefulSet {%s:%s}",
				sts.Namespace, sts.Name))
			return r.Client.Create(ctx, sts)
		}
		return err
	}
	return r.updateStatefulSet(ctx, s, foundSts, sts)
}

func (r *SeataServerReconciler) updateStatefulSet(ctx context.Context, s *seatav1alpha1.SeataServer,
	foundSts *appsv1.StatefulSet, sts *appsv1.StatefulSet) (err error) {
	logger := log.FromContext(ctx)

	logger.Info(fmt.Sprintf("Updating existing SeataServer StatefulSet {%s:%s}", foundSts.Namespace, foundSts.Name))
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
	} else if !s.Status.Synchronized {
		username, password := "seata", "seata"
		for _, env := range s.Spec.Env {
			if env.Name == "console.user.username" {
				username, err = seata.FetchEnvVar(ctx, r.Client, s, env)
				if err != nil {
					logger.Error(err, "Failed to fetch Env console.user.username")
				}
			}
			if env.Name == "console.user.password" {
				password, err = seata.FetchEnvVar(ctx, r.Client, s, env)
				if err != nil {
					logger.Error(err, "Failed to fetch Env console.user.password")
				}
			}
		}
		if err = seata.SyncRaftCluster(ctx, s, username, password); err != nil {
			logger.Error(err, "Failed to synchronize the raft cluster")
			s.Status.Synchronized = false
		} else {
			logger.Info("Successfully synchronized the raft cluster")
			s.Status.Synchronized = true
		}
	}
	return r.Client.Status().Update(ctx, s)
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

func setupDefaults(s *seatav1alpha1.SeataServer) {
	if s.Spec.Ports == (seatav1alpha1.Ports{}) {
		s.Spec.Ports.ConsolePort = 7091
		s.Spec.Ports.ServicePort = 8091
		s.Spec.Ports.RaftPort = 9091
	}
}
