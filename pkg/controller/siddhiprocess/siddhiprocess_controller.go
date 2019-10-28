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
	"time"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	artifact "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/artifact"
	deploymanager "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/deploymanager"
	messaging "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/messaging"
	parser "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/parser"
	siddhicontroller "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/siddhicontroller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

var log = logf.Log.WithName("siddhi")

// SPContainer holds siddhi apps
var SPContainer map[string][]deploymanager.Application

// CMContainer holds the config map name along with CM listner for listen changes of the CM
var CMContainer map[string]siddhicontroller.ConfigMapListner

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
	SPContainer = make(map[string][]deploymanager.Application)
	CMContainer = make(map[string]siddhicontroller.ConfigMapListner)
	ER = mgr.GetRecorder("siddhiprocess-controller")

	// Create a new controller
	c, err := controller.New("siddhiprocess-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	spPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			if _, ok := SPContainer[e.Meta.GetName()]; ok {
				delete(SPContainer, e.Meta.GetName())
			}
			return !e.DeleteStateUnknown
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*siddhiv1alpha2.SiddhiProcess)
			newObject := e.ObjectNew.(*siddhiv1alpha2.SiddhiProcess)
			if !oldObject.Spec.Equals(&newObject.Spec) {
				newObject.Status.CurrentVersion++
				newObject.Status.EventType = getEventType(UPDATE)
				return true
			}
			newObject.Status.EventType = getEventType(TIMER)
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			object := e.Object.(*siddhiv1alpha2.SiddhiProcess)
			object.Status.CurrentVersion = 0
			object.Status.PreviousVersion = 0
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	cmPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if _, ok := CMContainer[e.MetaNew.GetName()]; ok {
				cmListner := CMContainer[e.MetaNew.GetName()]
				cmListner.Changed = true
				CMContainer[e.MetaNew.GetName()] = cmListner
				return true
			}
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
	}

	// Watch for changes to primary resource SiddhiProcess
	err = c.Watch(&source.Kind{Type: &siddhiv1alpha2.SiddhiProcess{}}, &handler.EnqueueRequestForObject{}, spPredicate)
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Config Map and requeue the owner SiddhiProcess
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, cmPredicate)
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner SiddhiProcess
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha2.SiddhiProcess{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Services and requeue the owner SiddhiProcess
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha2.SiddhiProcess{},
	})
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
	sp := &siddhiv1alpha2.SiddhiProcess{}
	cm := &corev1.ConfigMap{}
	siddhiProcessName := request.NamespacedName
	siddhiProcessChanged := true
	err := rsp.client.Get(context.TODO(), request.NamespacedName, cm)
	if err == nil {
		cmListner := CMContainer[request.NamespacedName.Name]
		siddhiProcessName.Name = cmListner.SiddhiProcess
		siddhiProcessChanged = false
	}

	err = rsp.client.Get(context.TODO(), siddhiProcessName, sp)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	siddhiController := &siddhicontroller.SiddhiController{
		SiddhiProcess: sp,
		EventRecorder: ER,
		Logger:        logf.Log.WithValues("Namespace", sp.Namespace, "Name", sp.Name),
		KubeClient: artifact.KubeClient{
			Client: rsp.client,
			Scheme: rsp.scheme,
		},
	}
	siddhiController.UpdateDefaultConfigs()

	if sp.Status.EventType == getEventType(TIMER) {
		if SPContainer[sp.Name] != nil {
			terminate := siddhiController.CheckAvailableDeployments(SPContainer[sp.Name])
			if terminate {
				return reconcile.Result{Requeue: false}, nil
			}
		}
		return reconcile.Result{RequeueAfter: time.Second * time.Duration(ReconcileTime)}, nil
	}

	siddhiController.SetDefaultPendingState()
	if !siddhiProcessChanged {
		siddhiController.UpgradeVersion()
		sp = siddhiController.SiddhiProcess
	}
	siddhiApps, err := rsp.getSiddhiApps(sp)
	if err != nil {
		siddhiController.UpdateErrorStatus("AppReadError", err)
		return reconcile.Result{}, nil
	}
	parser := parser.Parser{
		Name:      sp.Name,
		Namespace: sp.Namespace,
		Apps:      siddhiApps,
		Image:     siddhiController.Image,
		Logger:    logf.Log.WithValues("Namespace", sp.Namespace, "Name", sp.Name),
		KubeClient: artifact.KubeClient{
			Client: rsp.client,
			Scheme: rsp.scheme,
		},
		SiddhiProcess:   sp,
		Env:             rsp.populateUserEnvs(sp),
		MessagingSystem: sp.Spec.MessagingSystem,
	}
	apps, err := rsp.populateSiddhiApps(sp, parser, siddhiController)
	if err != nil {
		siddhiController.UpdateErrorStatus("ParserFailed", err)
		return reconcile.Result{}, nil
	}
	sp = siddhiController.SiddhiProcess
	messaging := messaging.Messaging{
		KubeClient: artifact.KubeClient{
			Client: rsp.client,
			Scheme: rsp.scheme,
		},
		SiddhiProcess: sp,
	}
	messaging.CreateMessagingSystem(apps)
	if err != nil {
		siddhiController.UpdateErrorStatus("NATSCreationError", err)
		return reconcile.Result{}, err
	}

	siddhiController.CreateArtifacts(apps)
	siddhiController.CheckDeployments(apps)
	if !siddhiProcessChanged {
		cmListner := CMContainer[request.NamespacedName.Name]
		cmListner.Changed = false
		CMContainer[request.NamespacedName.Name] = cmListner
	}
	return reconcile.Result{RequeueAfter: time.Second * time.Duration(ReconcileTime)}, nil
}

// populateSiddhiApps function invoke parserApp function to retrieve relevant siddhi apps.
// Or else it will give you exixting siddhiApps list relevant to a particulat SiddhiProcess deployment.
func (rsp *ReconcileSiddhiProcess) populateSiddhiApps(
	sp *siddhiv1alpha2.SiddhiProcess,
	parser parser.Parser,
	siddhiController *siddhicontroller.SiddhiController,
) ([]deploymanager.Application, error) {
	var siddhiApps []deploymanager.Application
	var err error
	modified := false
	if sp.Status.CurrentVersion > sp.Status.PreviousVersion {
		modified = true
	}
	if modified {
		newApps, err := parser.Parse()
		if err != nil {
			return newApps, err
		}
		oldApps := SPContainer[sp.Name]
		err = siddhiController.CleanArtifacts(oldApps, newApps)
		if err != nil {
			return oldApps, err
		}
		SPContainer[sp.Name] = newApps
		siddhiApps = newApps
	} else {
		if _, ok := SPContainer[sp.Name]; ok {
			siddhiApps = SPContainer[sp.Name]
		} else {
			siddhiApps, err = parser.Parse()
			if err != nil {
				return siddhiApps, err
			}
			SPContainer[sp.Name] = siddhiApps
		}
	}
	return siddhiApps, err
}

// getSiddhiApps used to retrieve siddhi apps as a list of strigs.
func (rsp *ReconcileSiddhiProcess) getSiddhiApps(sp *siddhiv1alpha2.SiddhiProcess) (siddhiApps []string, err error) {
	for _, app := range sp.Spec.Apps {
		if app.ConfigMap != "" {
			configMap := &corev1.ConfigMap{}
			err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: app.ConfigMap, Namespace: sp.Namespace}, configMap)
			if err != nil {
				return
			}
			cmListner := siddhicontroller.ConfigMapListner{
				SiddhiProcess: sp.Name,
				Changed:       false,
			}
			CMContainer[app.ConfigMap] = cmListner
			for _, siddhiFileContent := range configMap.Data {
				siddhiApps = append(siddhiApps, siddhiFileContent)
			}
		}
		if app.Script != "" {
			siddhiApps = append(siddhiApps, app.Script)
		}
	}
	return
}

// PopulateUserEnvs returns a map for the ENVs in CRD
func (rsp *ReconcileSiddhiProcess) populateUserEnvs(sp *siddhiv1alpha2.SiddhiProcess) (envs map[string]string) {
	envs = make(map[string]string)
	for _, env := range sp.Spec.Container.Env {
		envs[env.Name] = env.Value
	}
	return envs
}
