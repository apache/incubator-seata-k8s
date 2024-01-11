/*
Copyright 1999-2019 Seata.io Group.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	seatav1alpha1 "github.com/apache/incubator-seata-k8s/api/v1alpha1"
)

// SeataServerReconciler reconciles a SeataServer object
type SeataServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.seata.io,resources=seataservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.seata.io,resources=seataservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.seata.io,resources=seataservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

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

	// create or update a service
	service := initService(s)
	if err := createOrUpdate(ctx, r, "Service", service, func() error {
		updateService(service, s)
		return controllerutil.SetControllerReference(s, service, r.Scheme)
	}, logger); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create resource Service(%v)", req.NamespacedName))
		return ctrl.Result{}, err
	}

	// create or update stateful sets
	statefulSet := initStatefulSet(s)
	if err := createOrUpdate(ctx, r, "StatefulSet", statefulSet, func() error {
		updateStatefulSet(ctx, statefulSet, s)
		return controllerutil.SetControllerReference(s, statefulSet, r.Scheme)
	}, logger); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create resource StatefulSet(%v)", req.NamespacedName))
		return ctrl.Result{}, err
	}

	if !s.Status.Deployed {
		s.Status.Deployed = true
		if err := r.Status().Update(ctx, s); err != nil {
			logger.Error(err, "Failed to update SeataServer/status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SeataServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&seatav1alpha1.SeataServer{}).
		Complete(r)
}

func setupDefaults(s *seatav1alpha1.SeataServer) {
	if s.Spec.Ports == (seatav1alpha1.Ports{}) {
		s.Spec.Ports.ConsolePort = 7091
		s.Spec.Ports.ServicePort = 8091
		s.Spec.Ports.RaftPort = 9091
	}
}

func createOrUpdate(ctx context.Context, r *SeataServerReconciler, kind string, object client.Object,
	f controllerutil.MutateFn, logger logr.Logger) error {
	key := client.ObjectKeyFromObject(object)
	status, err := controllerutil.CreateOrUpdate(ctx, r.Client, object, f)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to createOrUpdate object {%s:%v}", kind, key))
		return err
	}

	logger.Info(fmt.Sprintf("createOrUpdate object {%s:%v} : %s", kind, key, status))
	return nil
}

func makeLabels(name string) map[string]string {
	return map[string]string{"cr_name": name}
}
