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
	"strconv"
	"strings"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// deployApp creates a deployment according to the given SiddhiProcess specs.
// It create and mount volumes, create config maps, populates envs that needs to the deployment.
func (rsp *ReconcileSiddhiProcess) deployApp(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApp SiddhiApp,
	eventRecorder record.EventRecorder,
	configs Configs,
) (operationResult controllerutil.OperationResult, err error) {

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var strategy appsv1.DeploymentStrategy
	configParameter := ""
	siddhiParameter := ""
	siddhiAppsMap := make(map[string]string)
	labels := labelsForSiddhiProcess(siddhiApp.Name, configs)
	siddhiImage, siddhiHome, siddhiImageSecret := populateRunnerConfigs(sp, configs)
	containerPorts := siddhiApp.ContainerPorts

	if siddhiImageSecret != "" {
		secret := createLocalObjectReference(siddhiImageSecret)
		imagePullSecrets = append(imagePullSecrets, secret)
	}

	q := siddhiv1alpha2.PV{}
	if siddhiApp.PersistenceEnabled {
		if !sp.Spec.PV.Equals(&q) {
			pvcName := siddhiApp.Name + configs.PVCExt
			err = rsp.CreateOrUpdatePVC(sp, configs, pvcName)
			if err != nil {
				return
			}
			mountPath := ""
			mountPath, err = populateMountPath(sp, configs)
			if err != nil {
				return
			}
			volume, volumeMount := createPVCVolumes(pvcName, mountPath)
			volumes = append(volumes, volume)
			volumeMounts = append(volumeMounts, volumeMount)
		}
		deployYAMLCMName := sp.Name + configs.DepCMExt
		siddhiConfig := StatePersistenceConf
		if sp.Spec.SiddhiConfig != "" {
			siddhiConfig = sp.Spec.SiddhiConfig
		}
		data := map[string]string{
			deployYAMLCMName: siddhiConfig,
		}
		err = rsp.CreateOrUpdateCM(sp, deployYAMLCMName, data)
		if err != nil {
			return
		}
		mountPath := configs.SiddhiHome + configs.DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = configs.DepConfParameter + mountPath + deployYAMLCMName
	} else if sp.Spec.SiddhiConfig != "" {
		deployYAMLCMName := sp.Name + configs.DepCMExt
		data := map[string]string{
			deployYAMLCMName: sp.Spec.SiddhiConfig,
		}
		err = rsp.CreateOrUpdateCM(sp, deployYAMLCMName, data)
		if err != nil {
			return
		}
		mountPath := configs.SiddhiHome + configs.DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = configs.DepConfParameter + mountPath + deployYAMLCMName
	}

	maxUnavailable := intstr.IntOrString{
		Type:   Int,
		IntVal: configs.MaxUnavailable,
	}
	maxSurge := intstr.IntOrString{
		Type:   Int,
		IntVal: configs.MaxSurge,
	}
	rollingUpdate := appsv1.RollingUpdateDeployment{
		MaxUnavailable: &maxUnavailable,
		MaxSurge:       &maxSurge,
	}
	strategy = appsv1.DeploymentStrategy{
		Type:          appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &rollingUpdate,
	}

	if len(siddhiApp.Apps) > 0 {
		siddhiAppsCMName := siddhiApp.Name + strconv.Itoa(int(sp.Status.CurrentVersion))
		for k, v := range siddhiApp.Apps {
			key := k + configs.SiddhiExt
			siddhiAppsMap[key] = v
		}
		err = rsp.CreateOrUpdateCM(sp, siddhiAppsCMName, siddhiAppsMap)
		if err != nil {
			return
		}
		siddhiFilesPath := configs.SiddhiHome + configs.SiddhiFilesDir
		volume, volumeMount := createCMVolumes(siddhiAppsCMName, siddhiFilesPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		siddhiParameter = configs.AppConfParameter + siddhiFilesPath + " "
	} else {
		siddhiParameter = ParserParameter
	}

	userID := int64(802)
	operationResult, _ = rsp.CreateOrUpdateDeployment(
		sp,
		strings.ToLower(siddhiApp.Name),
		sp.Namespace,
		siddhiApp.Replicas,
		labels,
		siddhiImage,
		configs.ContainerName,
		[]string{configs.Shell},
		[]string{
			siddhiHome + configs.SiddhiBin + "/" + configs.SiddhiProfile + ".sh",
			siddhiParameter,
			configParameter,
		},
		containerPorts,
		volumeMounts,
		sp.Spec.Container.Env,
		corev1.SecurityContext{RunAsUser: &userID},
		corev1.PullAlways,
		imagePullSecrets,
		volumes,
		strategy,
		configs,
	)
	return
}

func (rsp *ReconcileSiddhiProcess) deployParser(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) (err error) {
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	siddhiApp := SiddhiApp{
		Name: sp.Name,
		ContainerPorts: []corev1.ContainerPort{
			corev1.ContainerPort{
				Name:          ParserName,
				ContainerPort: ParserPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ServiceEnabled: true,
		Replicas:       ParserReplicas,
	}
	_, err = rsp.deployApp(sp, siddhiApp, ER, configs)
	if err != nil {
		return
	}
	_, err = rsp.CreateOrUpdateService(sp, siddhiApp, configs)
	if err != nil {
		return
	}

	url := configs.ParserHTTP + sp.Name + "." + sp.Namespace + configs.ParserHealth
	reqLogger.Info("Waiting for parser", "deployment", sp.Name)
	err = waitForParser(url)
	if err != nil {
		return
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

// UpdateErrorStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Warning, Error
func (rsp *ReconcileSiddhiProcess) updateErrorStatus(
	sp *siddhiv1alpha2.SiddhiProcess,
	eventRecorder record.EventRecorder,
	status Status,
	reason string,
	er error,
) *siddhiv1alpha2.SiddhiProcess {
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	st := getStatus(status)
	s := sp
	sp.Status.Status = st

	if status == ERROR || status == WARNING {
		eventRecorder.Event(sp, getStatus(WARNING), reason, er.Error())
		if status == ERROR {
			reqLogger.Error(er, er.Error())
		} else {
			reqLogger.Info(er.Error())
		}
	}
	err := rsp.client.Status().Update(context.TODO(), sp)
	if err != nil {
		return s
	}
	return sp
}

// UpdateRunningStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Pending, Running
func (rsp *ReconcileSiddhiProcess) updateRunningStatus(
	sp *siddhiv1alpha2.SiddhiProcess,
	eventRecorder record.EventRecorder,
	status Status,
	reason string,
	message string,
) *siddhiv1alpha2.SiddhiProcess {
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	st := getStatus(status)
	s := sp
	sp.Status.Status = st
	if status == RUNNING {
		eventRecorder.Event(sp, getStatus(NORMAL), reason, message)
		reqLogger.Info(message)
	}
	err := rsp.client.Status().Update(context.TODO(), sp)
	if err != nil {
		return s
	}
	return sp
}

// UpdateReady update ready attribute of the CR object
// Ready attribute contains the number of deployments are complete and running out of requested deployments
func (rsp *ReconcileSiddhiProcess) updateReady(sp *siddhiv1alpha2.SiddhiProcess, available int, need int) *siddhiv1alpha2.SiddhiProcess {
	s := sp
	s.Status.Ready = strconv.Itoa(available) + "/" + strconv.Itoa(need)
	err := rsp.client.Status().Update(context.TODO(), sp)
	if err != nil {
		return s
	}
	return sp
}

// createArtifacts simply create all the k8s artifacts which needed in the siddhiApps list.
// This function creates deployment, service, and ingress. If ingress was available the it will update the ingress.
func (rsp *ReconcileSiddhiProcess) createArtifacts(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApps []SiddhiApp,
	configs Configs,
) *siddhiv1alpha2.SiddhiProcess {
	needDep := 0
	availableDep := 0
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	eventType := controllerutil.OperationResultNone
	if sp.Status.CurrentVersion == 0 {
		eventType = controllerutil.OperationResultCreated
	} else {
		if sp.Status.CurrentVersion > sp.Status.PreviousVersion {
			eventType = controllerutil.OperationResultUpdated
		} else {
			eventType = controllerutil.OperationResultNone
		}

	}
	for _, siddhiApp := range siddhiApps {
		if (eventType == controllerutil.OperationResultCreated) ||
			(eventType == controllerutil.OperationResultUpdated) {
			needDep++
			operationResult, err := rsp.deployApp(sp, siddhiApp, ER, configs)
			if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "AppDeploymentError", err)
				continue
			}
			if (eventType == controllerutil.OperationResultCreated) &&
				(operationResult == controllerutil.OperationResultCreated) {
				sp = rsp.updateRunningStatus(
					sp,
					ER,
					RUNNING,
					"DeploymentCreated",
					(siddhiApp.Name + " deployment created successfully"),
				)
			} else if (eventType == controllerutil.OperationResultUpdated) &&
				(operationResult == controllerutil.OperationResultUpdated) {
				sp = rsp.updateRunningStatus(
					sp,
					ER,
					RUNNING,
					"DeploymentUpdated",
					(siddhiApp.Name + " deployment updated successfully"),
				)
			}
			availableDep++
			if siddhiApp.ServiceEnabled {
				operationResult, err = rsp.CreateOrUpdateService(sp, siddhiApp, configs)
				if err != nil {
					sp = rsp.updateErrorStatus(sp, ER, WARNING, "ServiceCreationError", err)
					continue
				}
				if (eventType == controllerutil.OperationResultCreated) &&
					(operationResult == controllerutil.OperationResultCreated) {
					sp = rsp.updateRunningStatus(
						sp,
						ER,
						RUNNING,
						"ServiceCreated",
						(siddhiApp.Name + " service created successfully"),
					)
				} else if (eventType == controllerutil.OperationResultUpdated) &&
					(operationResult == controllerutil.OperationResultUpdated) {
					sp = rsp.updateRunningStatus(
						sp,
						ER,
						RUNNING,
						"ServiceUpdated",
						(siddhiApp.Name + " service updated successfully"),
					)
				}

				if configs.AutoCreateIngress {
					operationResult, err = rsp.CreateOrUpdateIngress(sp, siddhiApp, configs)
					if err != nil {
						sp = rsp.updateErrorStatus(sp, ER, ERROR, "IngressCreationError", err)
						continue
					}
					if (eventType == controllerutil.OperationResultCreated) &&
						(operationResult == controllerutil.OperationResultCreated) {
						reqLogger.Info("Ingress created", "Ingress.Name", configs.HostName)
					} else if (eventType == controllerutil.OperationResultUpdated) &&
						(operationResult == controllerutil.OperationResultUpdated) {
						reqLogger.Info("Ingress changed", "Ingress.Name", configs.HostName)
					}
				}
			}
		}
	}
	sp = rsp.syncVersion(sp)
	if (eventType == controllerutil.OperationResultCreated) ||
		(eventType == controllerutil.OperationResultUpdated) {
		sp = rsp.updateReady(sp, availableDep, needDep)
	}
	return sp
}

// checkDeployments function check the availability of deployments and the replications of the deployments.
func (rsp *ReconcileSiddhiProcess) checkDeployments(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApps []SiddhiApp,
) *siddhiv1alpha2.SiddhiProcess {
	for _, siddhiApp := range siddhiApps {
		deployment := &appsv1.Deployment{}
		err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err == nil && *deployment.Spec.Replicas != siddhiApp.Replicas {
			deployment.Spec.Replicas = &siddhiApp.Replicas
			err = rsp.client.Update(context.TODO(), deployment)
			if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "DeploymentUpdationError", err)
				continue
			}
		}
	}
	return sp
}

// populateSiddhiApps function invoke parserApp function to retrieve relevant siddhi apps.
// Or else it will give you exixting siddhiApps list relevant to a particulat SiddhiProcess deployment.
func (rsp *ReconcileSiddhiProcess) populateSiddhiApps(
	sp *siddhiv1alpha2.SiddhiProcess,
	configs Configs,
) (*siddhiv1alpha2.SiddhiProcess, []SiddhiApp, error) {
	var siddhiApps []SiddhiApp
	var err error
	modified := false
	if sp.Status.CurrentVersion > sp.Status.PreviousVersion {
		modified = true
	}
	if modified {
		siddhiApps, err = rsp.parseApp(sp, configs)
		if err != nil {
			return sp, siddhiApps, err
		}
		oldSiddhiApps := SPContainer[sp.Name]
		sp, err = rsp.cleanArtifacts(sp, configs, oldSiddhiApps, siddhiApps)
		if err != nil {
			return sp, siddhiApps, err
		}
		SPContainer[sp.Name] = siddhiApps
	} else {
		if _, ok := SPContainer[sp.Name]; ok {
			siddhiApps = SPContainer[sp.Name]
		} else {
			siddhiApps, err = rsp.parseApp(sp, configs)
			if err != nil {
				return sp, siddhiApps, err
			}
			SPContainer[sp.Name] = siddhiApps
		}
	}
	return sp, siddhiApps, err
}

// createMessagingSystem creates the messaging system if CR needed.
// If user specify only the messaging system type then this will creates the messaging system.
func (rsp *ReconcileSiddhiProcess) createMessagingSystem(sp *siddhiv1alpha2.SiddhiProcess, siddhiApps []SiddhiApp, configs Configs) (err error) {
	persistenceEnabled := false
	for _, siddhiApp := range siddhiApps {
		if siddhiApp.PersistenceEnabled {
			persistenceEnabled = true
			break
		}
	}
	if sp.Spec.MessagingSystem.TypeDefined() && sp.Spec.MessagingSystem.EmptyConfig() && persistenceEnabled {
		err = rsp.CreateNATS(sp, configs)
		if err != nil {
			return
		}
	}
	return
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
			cmListner := ConfigMapListner{
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

// syncVersion synchronize the siddhi process internal version number
// this simply assing Status.CurrentVersion value to the Status.PreviousVersion and update the siddhi process
// this funtionality used for version controlling inside a siddhi process
func (rsp *ReconcileSiddhiProcess) syncVersion(sp *siddhiv1alpha2.SiddhiProcess) *siddhiv1alpha2.SiddhiProcess {
	sp.Status.PreviousVersion = sp.Status.CurrentVersion
	_ = rsp.client.Status().Update(context.TODO(), sp)
	return sp
}

// upgradeVersion upgrade the siddhi process internal version number
func (rsp *ReconcileSiddhiProcess) upgradeVersion(sp *siddhiv1alpha2.SiddhiProcess) *siddhiv1alpha2.SiddhiProcess {
	sp.Status.CurrentVersion++
	_ = rsp.client.Status().Update(context.TODO(), sp)
	return sp
}

// cleanArtifacts function delete the k8s artifacts that are not relavant when user changes the existing SiddhiProcess
// When user change stateful siddhi process to stateless the unwanted artifacts will be deleted by this function
func (rsp *ReconcileSiddhiProcess) cleanArtifacts(
	sp *siddhiv1alpha2.SiddhiProcess,
	configs Configs,
	oldSiddhiApps []SiddhiApp,
	newSiddhiApps []SiddhiApp,
) (*siddhiv1alpha2.SiddhiProcess, error) {
	var err error
	oldSiddhiAppsLen := len(oldSiddhiApps)
	newSiddhiAppsLen := len(newSiddhiApps)
	if newSiddhiAppsLen < oldSiddhiAppsLen {
		for i := newSiddhiAppsLen; i < oldSiddhiAppsLen; i++ {
			artifactName := sp.Name + "-" + strconv.Itoa(i)
			err = rsp.DeleteDeployment(artifactName, sp.Namespace)
			if err != nil {
				return sp, err
			}
			sp = rsp.updateRunningStatus(
				sp,
				ER,
				RUNNING,
				"DeploymentDeleted",
				(artifactName + " deployment deleted successfully"),
			)

			err = rsp.DeleteService(artifactName, sp.Namespace)
			if err != nil {
				return sp, err
			}
			sp = rsp.updateRunningStatus(
				sp,
				ER,
				RUNNING,
				"ServiceDeleted",
				(artifactName + " service deleted successfully"),
			)

			pvcName := artifactName + configs.PVCExt
			err = rsp.DeletePVC(pvcName, sp.Namespace)
			if err != nil {
				return sp, err
			}

			cmName := artifactName + strconv.Itoa(int(sp.Status.PreviousVersion))
			err = rsp.DeleteConfigMap(cmName, sp.Namespace)
			if err != nil {
				return sp, err
			}
		}
	}
	return sp, err
}

// cleanParser simply remove the parser deployment and service created by the operator
func (rsp *ReconcileSiddhiProcess) cleanParser(
	sp *siddhiv1alpha2.SiddhiProcess,
) (err error) {
	err = rsp.DeleteDeployment(sp.Name, sp.Namespace)
	if err != nil {
		return
	}
	err = rsp.DeleteService(sp.Name, sp.Namespace)
	if err != nil {
		return
	}
	return
}
