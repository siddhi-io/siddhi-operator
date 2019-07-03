/*
 * Copyright (c) 2019 WSO2 Inc. (http:www.wso2.org) All Rights Reserved.
 *
 * WSO2 Inc. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http:www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package siddhiprocess

import (
	"context"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_siddhiprocess")

// SPContainer holds siddhi apps
var SPContainer map[string][]SiddhiApp

// ER recoder
var ER record.EventRecorder

// Add creates a new SiddhiProcess Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSiddhiProcess{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with rsp as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	SPContainer = make(map[string][]SiddhiApp)
	ER = mgr.GetRecorder("siddhiprocess-controller")

	// Create a new controller
	c, err := controller.New("siddhiprocess-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	sp := &source.Kind{Type: &siddhiv1alpha2.SiddhiProcess{}}
	h := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha2.SiddhiProcess{},
	}
	pred := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			if _, ok := SPContainer[e.Meta.GetName()]; ok {
				delete(SPContainer, e.Meta.GetName())
			}
			return !e.DeleteStateUnknown
		},
	}

	// Watch for Pod events.
	err = c.Watch(sp, h, pred)
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SiddhiProcess
	err = c.Watch(&source.Kind{Type: &siddhiv1alpha2.SiddhiProcess{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner SiddhiProcess
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha2.SiddhiProcess{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSiddhiProcess{}

// ReconcileSiddhiProcess reconciles a SiddhiProcess object
type ReconcileSiddhiProcess struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SiddhiProcess object and makes changes based on the state read
// and what is in the SiddhiProcess.Spec
func (rsp *ReconcileSiddhiProcess) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SiddhiProcess")

	sp := &siddhiv1alpha2.SiddhiProcess{}
	err := rsp.client.Get(context.TODO(), request.NamespacedName, sp)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	configs := rsp.Configurations(sp)
	siddhiApps, err := rsp.populateSiddhiApps(sp, configs)
	if err != nil {
		sp = rsp.updateErrorStatus(sp, ER, ERROR, "ParserFailed", err)
		return reconcile.Result{}, nil
	}

	err = rsp.createMessagingSystem(sp, configs)
	if err != nil {
		sp = rsp.updateErrorStatus(sp, ER, ERROR, "NATSCreationError", err)
		return reconcile.Result{}, err
	}

	sp = rsp.createArtifacts(sp, siddhiApps, configs)
	sp = rsp.checkDeployments(sp, siddhiApps)
	return reconcile.Result{Requeue: false}, nil
}
