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

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Status of a Siddhi process
type Status int

// Tye of status
const (
	PENDING Status = iota
	READY
	RUNNING
	ERROR
)

var log = logf.Log.WithName("controller_siddhiprocess")

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
	// Create a new controller
	c, err := controller.New("siddhiprocess-controller", mgr, controller.Options{Reconciler: r})
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
func (reconcileSiddhiProcess *ReconcileSiddhiProcess) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SiddhiProcess")
	reqLogger.Info(request.Namespace)
	
	// Fetch the SiddhiProcess instance
	siddhiProcess := &siddhiv1alpha1.SiddhiProcess{}
	err := reconcileSiddhiProcess.client.Get(context.TODO(), request.NamespacedName, siddhiProcess)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	
	var operatorEnvs map[string]string
	operatorEnvs = make(map[string]string)
	operatorDeployment := &appsv1.Deployment{}
	err = reconcileSiddhiProcess.client.Get(context.TODO(), types.NamespacedName{Name: "siddhi-operator", Namespace: siddhiProcess.Namespace}, operatorDeployment)
	if err != nil{
		reqLogger.Info("siddhi-operator deployment not found")
	} else {
		operatorEnvs = reconcileSiddhiProcess.populateOperatorEnvs(operatorDeployment)
	}

	var siddhiApp SiddhiApp
	siddhiApp, err = reconcileSiddhiProcess.parseSiddhiApp(siddhiProcess)
	if err != nil{
		reqLogger.Error(err, err.Error())
		return reconcile.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	deployment := &appsv1.Deployment{}
	err = reconcileSiddhiProcess.client.Get(context.TODO(), types.NamespacedName{Name: siddhiProcess.Name, Namespace: siddhiProcess.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		siddhiProcess.Status.Status = getStatus(PENDING)
		err = reconcileSiddhiProcess.client.Status().Update(context.TODO(), siddhiProcess)
		if err != nil {
			reqLogger.Error(err, "Failed to update SiddhiProcess status")
			siddhiProcess.Status.Status = getStatus(ERROR)
		}
		siddhiDeployment, err := reconcileSiddhiProcess.deploymentForSiddhiProcess(siddhiProcess, siddhiApp, operatorEnvs)
		if err != nil{
			reqLogger.Error(err, err.Error())
			siddhiProcess.Status.Status = getStatus(ERROR)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", siddhiDeployment.Namespace, "Deployment.Name", siddhiDeployment.Name)
		err = reconcileSiddhiProcess.client.Create(context.TODO(), siddhiDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", siddhiDeployment.Namespace, "Deployment.Name", siddhiDeployment.Name)
			siddhiProcess.Status.Status = getStatus(ERROR)
			return reconcile.Result{}, err
		}
		
		// Deployment created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		siddhiProcess.Status.Status = getStatus(ERROR)
		return reconcile.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := int32(1)
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err = reconcileSiddhiProcess.client.Update(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

	service := &corev1.Service{}
	err = reconcileSiddhiProcess.client.Get(context.TODO(), types.NamespacedName{Name: siddhiProcess.Name, Namespace: siddhiProcess.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		siddhiService := reconcileSiddhiProcess.serviceForSiddhiProcess(siddhiProcess, siddhiApp, operatorEnvs)
		reqLogger.Info("Creating a new Service", "Service.Namespace", siddhiService.Namespace, "Service.Name", siddhiService.Name)
		err = reconcileSiddhiProcess.client.Create(context.TODO(), siddhiService)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Service", "Service.Namespace", siddhiService.Namespace, "Service.Name", siddhiService.Name)
			siddhiProcess.Status.Status = getStatus(ERROR)
			return reconcile.Result{}, err
		}
		// Service created successfully - return and requeue
		siddhiProcess.Status.Status = getStatus(RUNNING)
		err = reconcileSiddhiProcess.client.Status().Update(context.TODO(), siddhiProcess)
		if err != nil {
			reqLogger.Error(err, "Failed to update SiddhiProcess status")
			siddhiProcess.Status.Status = getStatus(ERROR)
		}
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service")
		siddhiProcess.Status.Status = getStatus(ERROR)
		return reconcile.Result{}, err
	}

	createIngress := true
	if (operatorEnvs["AUTO_INGRESS_CREATION"] != "") && (operatorEnvs["AUTO_INGRESS_CREATION"] != "false") {
		createIngress = true
	} else {
		createIngress = false
	}

	if createIngress{
		ingress := &extensionsv1beta1.Ingress{}
		err = reconcileSiddhiProcess.client.Get(context.TODO(), types.NamespacedName{Name: "siddhi", Namespace: siddhiProcess.Namespace}, ingress)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Ingress
			siddhiIngress := reconcileSiddhiProcess.loadBalancerForSiddhiProcess(siddhiProcess, siddhiApp)
			reqLogger.Info("Creating a new Ingress", "Ingress.Namespace", siddhiIngress.Namespace, "Ingress.Name", siddhiIngress.Name)
			err = reconcileSiddhiProcess.client.Create(context.TODO(), siddhiIngress)
			if err != nil {
				reqLogger.Error(err, "Failed to create new Ingress", "Ingress.Namespace", siddhiIngress.Namespace, "Ingress.Name", siddhiIngress.Name)
				return reconcile.Result{}, err
			}
			// Ingress created successfully - return and requeue
			reqLogger.Info("Ingress created successfully")
			return reconcile.Result{Requeue: true}, nil
		} else if err == nil{
			siddhiIngress := reconcileSiddhiProcess.updatedLoadBalancerForSiddhiProcess(siddhiProcess, ingress, siddhiApp)
			reqLogger.Info("Updating a new Ingress", "Ingress.Namespace", siddhiIngress.Namespace, "Ingress.Name", siddhiIngress.Name)
			err = reconcileSiddhiProcess.client.Update(context.TODO(), siddhiIngress)
			if err != nil {
				reqLogger.Error(err, "Failed to updated new Ingress", "Ingress.Namespace", siddhiIngress.Namespace, "Ingress.Name", siddhiIngress.Name)
				return reconcile.Result{}, err
			}
			// Ingress updated successfully - return and requeue
			reqLogger.Info("Ingress updated successfully")
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil{
			reqLogger.Error(err, "Failed to get Ingress")
			return reconcile.Result{}, err
		}
	}


	
	// Update the SiddhiProcess status with the pod names
	// List the pods for this siddhiProcess's deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForSiddhiProcess(siddhiProcess.Name, operatorEnvs))
	listOps := &client.ListOptions{Namespace: siddhiProcess.Namespace, LabelSelector: labelSelector}
	err = reconcileSiddhiProcess.client.List(context.TODO(), listOps, podList)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods", "SiddhiProcess.Namespace", siddhiProcess.Namespace, "SiddhiProcess.Name", siddhiProcess.Name)
		return reconcile.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, siddhiProcess.Status.Nodes) {
		siddhiProcess.Status.Nodes = podNames
		err := reconcileSiddhiProcess.client.Status().Update(context.TODO(), siddhiProcess)
		if err != nil {
			reqLogger.Error(err, "Failed to update SiddhiProcess status")
		}
	}
	return reconcile.Result{}, err
}

// labelsForSiddhiProcess returns the labels for selecting the resources
// belonging to the given siddhiProcess CR name.
func labelsForSiddhiProcess(appName string, operatorEnvs map[string]string) map[string]string {
	operatorName := "siddhi-operator"
	operatorVersion := "0.1.0"
	if operatorEnvs["OPERATOR_NAME"] != "" {
		operatorName = operatorEnvs["OPERATOR_NAME"]
	}
	if operatorEnvs["OPERATOR_VERSION"] != "" {
		operatorVersion = operatorEnvs["OPERATOR_VERSION"]
	}
	return map[string]string{
		"siddhi.io/name": "SiddhiProcess",
		"siddhi.io/instance": appName,
		"siddhi.io/version": operatorVersion,
		"siddhi.io/part-of": operatorName,
	}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// Status array
var status = []string{
	"Pending",
	"Ready",
	"Running",
	"Error",
}

// getStatus return relevant status to a given int
func getStatus(n Status) string {
	return status[n]
}