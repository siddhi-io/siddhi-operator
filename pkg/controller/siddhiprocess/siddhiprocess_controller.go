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
	"reflect"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	SPContainer = make(map[string][]SiddhiApp)
	ER = mgr.GetRecorder("siddhiprocess-controller")

	// Create a new controller
	c, err := controller.New("siddhiprocess-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	sp := &source.Kind{Type: &siddhiv1alpha1.SiddhiProcess{}}
	h := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha1.SiddhiProcess{},
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
	err = c.Watch(&source.Kind{Type: &siddhiv1alpha1.SiddhiProcess{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner SiddhiProcess
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &siddhiv1alpha1.SiddhiProcess{},
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
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (rsp *ReconcileSiddhiProcess) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SiddhiProcess")
	em := ""

	sp := &siddhiv1alpha1.SiddhiProcess{}
	err := rsp.client.Get(context.TODO(), request.NamespacedName, sp)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	configs := Configurations()

	var operatorEnvs map[string]string
	operatorEnvs = make(map[string]string)
	operatorDeployment := &appsv1.Deployment{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.OperatorName, Namespace: sp.Namespace}, operatorDeployment)
	if err != nil {
		sp = rsp.updateStatus(WARNING, "OperatorNotFound", err.Error(), ER, err, sp)
	} else {
		operatorEnvs = rsp.populateOperatorEnvs(operatorDeployment)
	}

	var siddhiApps []SiddhiApp
	if _, ok := SPContainer[sp.Name]; ok {
		siddhiApps = SPContainer[sp.Name]
	} else {
		sp = rsp.updateStatus(PENDING, "", "", ER, nil, sp)
		siddhiApps, err = rsp.parseApp(sp, configs)
		SPContainer[sp.Name] = siddhiApps
		if err != nil {
			sp = rsp.updateStatus(ERROR, "ParserFailed", err.Error(), ER, err, sp)
			return reconcile.Result{}, err
		}
	}

	if sp.Spec.DeploymentConfigs.Mode == Failover {
		sp = rsp.updateType(Failover, sp)
		ms := siddhiv1alpha1.MessagingSystem{}
		if sp.Spec.DeploymentConfigs.MessagingSystem.Equals(&ms) {
			err = rsp.createNATS(sp, configs, operatorDeployment)
			if err != nil {
				sp = rsp.updateStatus(WARNING, "NATSCreationError", "Failed automatic NATS creation", ER, err, sp)
			}
		}
	} else {
		sp = rsp.updateType(Default, sp)
	}

	needDep := 0
	availableDep := 0
	for _, siddhiApp := range siddhiApps {
		needDep++
		var siddhiDeployment *appsv1.Deployment
		deployment := &appsv1.Deployment{}
		err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err != nil && errors.IsNotFound(err) {
			siddhiDeployment, sp, err = rsp.deployApp(sp, siddhiApp, operatorEnvs, configs, ER)
			if err != nil {
				sp = rsp.updateStatus(ERROR, "DeployError", err.Error(), ER, err, sp)
				continue
			}
			err = rsp.client.Create(context.TODO(), siddhiDeployment)
			if err != nil {
				em = "Failed to create a new deployment : " + siddhiDeployment.Name
				sp = rsp.updateStatus(ERROR, "DeploymentCreationError", em, ER, err, sp)
				continue
			}
			availableDep++
			sp = rsp.updateStatus(RUNNING, "DeploymentCreated", (siddhiDeployment.Name + " deployment created successfully"), ER, nil, sp)
		} else if err != nil {
			em = "Failed to get the deployment : " + strings.ToLower(siddhiApp.Name)
			sp = rsp.updateStatus(ERROR, "DeploymentNotFound", em, ER, err, sp)
			continue
		} else {
			availableDep++
		}

		if siddhiApp.CreateSVC {
			service := &corev1.Service{}
			err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: siddhiApp.Name, Namespace: sp.Namespace}, service)
			if err != nil && errors.IsNotFound(err) {
				siddhiService := rsp.createService(sp, siddhiApp, operatorEnvs, configs)
				err = rsp.client.Create(context.TODO(), siddhiService)
				if err != nil {
					em = "Failed to create a new service : " + siddhiService.Name
					sp = rsp.updateStatus(WARNING, "ServiceCreationError", em, ER, err, sp)
					continue
				}
				sp = rsp.updateStatus(RUNNING, "ServiceCreated", (siddhiService.Name + " service created successfully"), ER, nil, sp)
			} else if err != nil {
				em = "Failed to find the service : " + siddhiApp.Name
				sp = rsp.updateStatus(ERROR, "ServiceNotFound", em, ER, err, sp)
				continue
			}

			createIngress := true
			if (operatorEnvs["AUTO_INGRESS_CREATION"] != "") && (operatorEnvs["AUTO_INGRESS_CREATION"] != "false") {
				createIngress = true
			} else {
				createIngress = false
			}
			if createIngress {
				ingress := &extensionsv1beta1.Ingress{}
				err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.HostName, Namespace: sp.Namespace}, ingress)
				if err != nil && errors.IsNotFound(err) {
					siddhiIngress := rsp.createIngress(sp, siddhiApp, configs, operatorDeployment)
					err = rsp.client.Create(context.TODO(), siddhiIngress)
					if err != nil {
						em = "Failed to create a new ingress : " + siddhiIngress.Name
						sp = rsp.updateStatus(ERROR, "IngressCreationError", em, ER, err, sp)
						continue
					}
					reqLogger.Info("New ingress created successfully", "Ingress.Name", siddhiIngress.Name)
				} else if err != nil {
					siddhiIngress := rsp.updateIngress(sp, ingress, siddhiApp, configs)
					err = rsp.client.Update(context.TODO(), siddhiIngress)
					if err != nil {
						em = "Failed to updated the ingress : " + siddhiIngress.Name
						sp = rsp.updateStatus(ERROR, "IngressUpdationError", em, ER, err, sp)
						continue
					}
					reqLogger.Info("Ingress updated successfully", "Ingress.Name", siddhiIngress.Name)
				}
			}
		}
	}
	sp = rsp.updateReady(availableDep, needDep, sp)

	for _, siddhiApp := range siddhiApps {
		deployment := &appsv1.Deployment{}
		err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err == nil && *deployment.Spec.Replicas != configs.DeploymentSize {
			deployment.Spec.Replicas = &configs.DeploymentSize
			err = rsp.client.Update(context.TODO(), deployment)
			if err != nil {
				em = "Failed to update deployment : " + deployment.Name
				sp = rsp.updateStatus(ERROR, "DeploymentUpdationError", em, ER, err, sp)
				return reconcile.Result{}, nil
			}
		}
	}

	// Update the SiddhiProcess status with the pod names
	// List the pods for this sp's deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForSiddhiProcess(sp.Name, operatorEnvs, configs))
	listOps := &client.ListOptions{Namespace: sp.Namespace, LabelSelector: labelSelector}
	err = rsp.client.List(context.TODO(), listOps, podList)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods", "SiddhiProcess.Name", sp.Name)
		return reconcile.Result{}, nil
	}
	podNames := podNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, sp.Status.Nodes) {
		sp.Status.Nodes = podNames
		err := rsp.client.Status().Update(context.TODO(), sp)
		if err != nil {
			reqLogger.Error(err, "Failed to update SiddhiProcess status")
		}
	}

	return reconcile.Result{}, nil
}
