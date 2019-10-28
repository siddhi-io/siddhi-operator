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

package siddhicontroller

import (
	"context"
	"os"
	"strconv"
	"strings"

	logr "github.com/go-logr/logr"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	artifact "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/artifact"
	deploymanager "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/deploymanager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SiddhiController control all deployments of the SiddhiProcess
type SiddhiController struct {
	SiddhiProcess     *siddhiv1alpha2.SiddhiProcess
	EventRecorder     record.EventRecorder
	Logger            logr.Logger
	KubeClient        artifact.KubeClient
	Image             deploymanager.Image
	AutoCreateIngress bool
	TLS               string
}

// ConfigMapListner holds the change details of a config map
type ConfigMapListner struct {
	SiddhiProcess string `json:"siddhiProcess"`
	Changed       bool   `json:"changed"`
}

// UpdateErrorStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Warning, Error
func (sc *SiddhiController) UpdateErrorStatus(
	reason string,
	er error,
) {
	st := getStatus(ERROR)
	s := sc.SiddhiProcess
	sc.SiddhiProcess.Status.Status = st
	sc.EventRecorder.Event(sc.SiddhiProcess, getStatus(WARNING), reason, er.Error())
	sc.Logger.Error(er, er.Error())
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// UpdateWarningStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Warning, Error
func (sc *SiddhiController) UpdateWarningStatus(
	reason string,
	er error,
) {
	st := getStatus(WARNING)
	s := sc.SiddhiProcess
	sc.SiddhiProcess.Status.Status = st
	sc.EventRecorder.Event(sc.SiddhiProcess, getStatus(WARNING), reason, er.Error())
	sc.Logger.Info(er.Error())
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// UpdateRunningStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
func (sc *SiddhiController) UpdateRunningStatus(
	reason string,
	message string,
) {
	st := getStatus(RUNNING)
	s := sc.SiddhiProcess
	sc.SiddhiProcess.Status.Status = st
	if reason != "" && message != "" {
		sc.EventRecorder.Event(sc.SiddhiProcess, getStatus(NORMAL), reason, message)
		sc.Logger.Info(message)
	}
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// UpdatePendingStatus update the status of the CR object to Pending status
func (sc *SiddhiController) UpdatePendingStatus() {
	st := getStatus(PENDING)
	s := sc.SiddhiProcess
	sc.SiddhiProcess.Status.Status = st
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// UpdateUpdatingtatus update the status of the CR object to Updating status
func (sc *SiddhiController) UpdateUpdatingtatus() {
	st := getStatus(UPDATING)
	s := sc.SiddhiProcess
	sc.SiddhiProcess.Status.Status = st
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// UpdateReady update ready attribute of the CR object
// Ready attribute contains the number of deployments are complete and running out of requested deployments
func (sc *SiddhiController) UpdateReady(available int, need int) {
	s := sc.SiddhiProcess
	s.Status.Ready = strconv.Itoa(available) + "/" + strconv.Itoa(need)
	err := sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
	if err != nil {
		sc.SiddhiProcess = s
	}
}

// CreateArtifacts simply create all the k8s artifacts which needed in the siddhiApps list.
// This function creates deployment, service, and ingress. If ingress was available the it will update the ingress.
func (sc *SiddhiController) CreateArtifacts(applications []deploymanager.Application) {
	needDep := 0
	availableDep := 0
	eventType := getEventType(
		sc.SiddhiProcess.Status.CurrentVersion,
		sc.SiddhiProcess.Status.PreviousVersion,
	)

	if (eventType == controllerutil.OperationResultUpdated) && (sc.GetDeploymentCount(applications) > 0) {
		sc.UpdateUpdatingtatus()
	}

	for _, application := range applications {
		if (eventType == controllerutil.OperationResultCreated) ||
			(eventType == controllerutil.OperationResultUpdated) {
			deployManeger := deploymanager.DeployManager{
				Application:   application,
				KubeClient:    sc.KubeClient,
				Image:         sc.Image,
				SiddhiProcess: sc.SiddhiProcess,
			}
			needDep++
			operationResult, err := deployManeger.Deploy()
			if err != nil {
				sc.UpdateErrorStatus("AppDeploymentError", err)
				continue
			}
			if (eventType == controllerutil.OperationResultCreated) &&
				(operationResult == controllerutil.OperationResultCreated) {
				sc.UpdateRunningStatus(
					"DeploymentCreated",
					(application.Name + " deployment created successfully"),
				)
			} else if (eventType == controllerutil.OperationResultUpdated) &&
				(operationResult == controllerutil.OperationResultUpdated) {
				sc.UpdateRunningStatus(
					"DeploymentUpdated",
					(application.Name + " deployment updated successfully"),
				)
			}
			availableDep++
			if application.ServiceEnabled {
				operationResult, err = sc.KubeClient.CreateOrUpdateService(
					application.Name,
					sc.SiddhiProcess.Namespace,
					application.ContainerPorts,
					deployManeger.Labels,
					sc.SiddhiProcess,
				)
				if err != nil {
					sc.UpdateErrorStatus("ServiceCreationError", err)
					continue
				}
				if (eventType == controllerutil.OperationResultCreated) &&
					(operationResult == controllerutil.OperationResultCreated) {
					sc.UpdateRunningStatus(
						"ServiceCreated",
						(application.Name + " service created successfully"),
					)
				} else if (eventType == controllerutil.OperationResultUpdated) &&
					(operationResult == controllerutil.OperationResultUpdated) {
					sc.UpdateRunningStatus(
						"ServiceUpdated",
						(application.Name + " service updated successfully"),
					)
				}

				if sc.AutoCreateIngress {
					operationResult, err = sc.KubeClient.CreateOrUpdateIngress(
						sc.SiddhiProcess.Namespace,
						application.Name,
						sc.TLS,
						application.ContainerPorts,
					)
					if err != nil {
						sc.UpdateErrorStatus("IngressCreationError", err)
						continue
					}
					if (eventType == controllerutil.OperationResultCreated) &&
						(operationResult == controllerutil.OperationResultCreated) {
						sc.Logger.Info("Ingress created", "Ingress.Name", artifact.IngressName)
					} else if (eventType == controllerutil.OperationResultUpdated) &&
						(operationResult == controllerutil.OperationResultUpdated) {
						sc.Logger.Info("Ingress changed", "Ingress.Name", artifact.IngressName)
					}
				}
			}
		}
	}
	sc.SyncVersion()
	if (eventType == controllerutil.OperationResultCreated) ||
		(eventType == controllerutil.OperationResultUpdated) {
		sc.UpdateReady(0, needDep)
	}
	return
}

// CheckDeployments function check the availability of deployments and the replications of the deployments.
func (sc *SiddhiController) CheckDeployments(applications []deploymanager.Application) {
	for _, application := range applications {
		deployment := &appsv1.Deployment{}
		err := sc.KubeClient.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      strings.ToLower(application.Name),
				Namespace: sc.SiddhiProcess.Namespace,
			},
			deployment,
		)
		if err == nil && *deployment.Spec.Replicas != application.Replicas {
			deployment.Spec.Replicas = &application.Replicas
			err = sc.KubeClient.Client.Update(context.TODO(), deployment)
			if err != nil {
				sc.UpdateErrorStatus("DeploymentUpdationError", err)
				continue
			}
		}
	}
}

// CheckAvailableDeployments function check the availability of deployments and the replications of the deployments.
func (sc *SiddhiController) CheckAvailableDeployments(applications []deploymanager.Application) (terminate bool) {
	availableDeployments := 0
	needDeployments := 0
	for _, application := range applications {
		deployment := &appsv1.Deployment{}
		err := sc.KubeClient.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      strings.ToLower(application.Name),
				Namespace: sc.SiddhiProcess.Namespace,
			},
			deployment,
		)
		needDeployments++
		if err == nil && &deployment.Status.AvailableReplicas != nil {
			availableDeployments += int(deployment.Status.AvailableReplicas)
		}
	}
	sc.UpdateReady(availableDeployments, needDeployments)
	if needDeployments == availableDeployments {
		terminate = true
		sc.UpdateRunningStatus("", "")
		return
	}
	terminate = false
	return
}

// SyncVersion synchronize the siddhi process internal version number
// this simply assing Status.CurrentVersion value to the Status.PreviousVersion and update the siddhi process
// this funtionality used for version controlling inside a siddhi process
func (sc *SiddhiController) SyncVersion() {
	sc.SiddhiProcess.Status.PreviousVersion = sc.SiddhiProcess.Status.CurrentVersion
	_ = sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
}

// UpgradeVersion upgrade the siddhi process internal version number
func (sc *SiddhiController) UpgradeVersion() {
	sc.SiddhiProcess.Status.CurrentVersion++
	_ = sc.KubeClient.Client.Status().Update(context.TODO(), sc.SiddhiProcess)
}

// CleanArtifacts function delete the k8s artifacts that are not relavant when user changes the existing SiddhiProcess
// When user change stateful siddhi process to stateless the unwanted artifacts will be deleted by this function
func (sc *SiddhiController) CleanArtifacts(
	oldApps []deploymanager.Application,
	newApps []deploymanager.Application,
) (err error) {
	oldAppsLen := len(oldApps)
	newAppsLen := len(newApps)
	if newAppsLen < oldAppsLen {
		for i := newAppsLen; i < oldAppsLen; i++ {
			artifactName := sc.SiddhiProcess.Name + "-" + strconv.Itoa(i)
			err = sc.KubeClient.DeleteDeployment(artifactName, sc.SiddhiProcess.Namespace)
			if err != nil {
				return
			}
			sc.UpdateRunningStatus(
				"DeploymentDeleted",
				(artifactName + " deployment deleted successfully"),
			)

			err = sc.KubeClient.DeleteService(artifactName, sc.SiddhiProcess.Namespace)
			if err != nil {
				return
			}
			sc.UpdateRunningStatus(
				"ServiceDeleted",
				(artifactName + " service deleted successfully"),
			)

			pvcName := artifactName + deploymanager.PVCExtension
			err = sc.KubeClient.DeletePVC(pvcName, sc.SiddhiProcess.Namespace)
			if err != nil {
				return
			}

			cmName := artifactName + strconv.Itoa(int(sc.SiddhiProcess.Status.PreviousVersion))
			err = sc.KubeClient.DeleteConfigMap(cmName, sc.SiddhiProcess.Namespace)
			if err != nil {
				return
			}
		}
	}
	return
}

func getEventType(currentVersion int64, previousVersion int64) (eventType controllerutil.OperationResult) {
	if currentVersion == 0 {
		eventType = controllerutil.OperationResultCreated
	} else {
		if currentVersion > previousVersion {
			eventType = controllerutil.OperationResultUpdated
		} else {
			eventType = controllerutil.OperationResultNone
		}
	}
	return
}

// UpdateDefaultConfigs updates the default configs of Image, TLS, and ingress creation
func (sc *SiddhiController) UpdateDefaultConfigs() {
	cmName := OperatorCMName
	env := os.Getenv("OPERATOR_CONFIGMAP")
	if env != "" {
		cmName = env
	}
	sc.Image.Home = SiddhiHome
	sc.Image.Name = SiddhiImage
	sc.Image.Profile = SiddhiProfile
	sc.AutoCreateIngress = AutoCreateIngress

	configMap := &corev1.ConfigMap{}
	err := sc.KubeClient.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: cmName, Namespace: sc.SiddhiProcess.Namespace},
		configMap,
	)
	if err == nil {
		if configMap.Data["siddhiHome"] != "" {
			sc.Image.Home = configMap.Data["siddhiHome"]
		}

		if configMap.Data["siddhiImage"] != "" {
			sc.Image.Name = configMap.Data["siddhiImage"]
		}

		if configMap.Data["siddhiImageSecret"] != "" {
			sc.Image.Secret = configMap.Data["siddhiImageSecret"]
		}

		if configMap.Data["siddhiProfile"] != "" {
			sc.Image.Profile = configMap.Data["siddhiProfile"]
		}

		if configMap.Data["autoIngressCreation"] != "" {
			if configMap.Data["autoIngressCreation"] == "true" {
				sc.AutoCreateIngress = true
			} else {
				sc.AutoCreateIngress = false
			}
		}

		if configMap.Data["ingressTLS"] != "" {
			sc.TLS = configMap.Data["ingressTLS"]
		}

	}
	if sc.SiddhiProcess.Spec.Container.Image != "" {
		sc.Image.Name = sc.SiddhiProcess.Spec.Container.Image
	}
	if sc.SiddhiProcess.Spec.ImagePullSecret != "" {
		sc.Image.Secret = sc.SiddhiProcess.Spec.ImagePullSecret
	}
}

// GetDeploymentCount returns the deployment count to a given set of applications
func (sc *SiddhiController) GetDeploymentCount(applications []deploymanager.Application) (count int) {
	count = 0
	isDeployExists := false
	for _, application := range applications {
		_, isDeployExists = sc.KubeClient.GetDeployment(application.Name, sc.SiddhiProcess.Namespace)
		if isDeployExists {
			count = count + 1
		}
	}
	return
}

// SetDefaultPendingState set the default state of a SiddhiProcess object to Pending
func (sc *SiddhiController) SetDefaultPendingState() {
	eventType := getEventType(
		sc.SiddhiProcess.Status.CurrentVersion,
		sc.SiddhiProcess.Status.PreviousVersion,
	)
	if (eventType == controllerutil.OperationResultCreated) && (sc.SiddhiProcess.Status.Status == "") {
		sc.UpdatePendingStatus()
	}
}
