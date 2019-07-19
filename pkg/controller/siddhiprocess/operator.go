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
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
)

// deployApp creates a deployment according to the given SiddhiProcess specs.
// It create and mount volumes, create config maps, populates envs that needs to the deployment.
func (rsp *ReconcileSiddhiProcess) deployApp(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApp SiddhiApp,
	eventRecorder record.EventRecorder,
	configs Configs,
) (err error) {

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var strategy appsv1.DeploymentStrategy
	configMapData := make(map[string]string)
	labels := labelsForSiddhiProcess(siddhiApp.Name, configs)
	siddhiRunnerImage, siddhiHome, siddhiImageSecret := populateRunnerConfigs(sp, configs)
	containerPorts := siddhiApp.ContainerPorts

	if siddhiImageSecret != "" {
		secret := createLocalObjectReference(siddhiImageSecret)
		imagePullSecrets = append(imagePullSecrets, secret)
	}

	configParameter := ""
	q := siddhiv1alpha2.PV{}
	if siddhiApp.PersistenceEnabled {
		if !sp.Spec.PV.Equals(&q) {
			pvcName := siddhiApp.Name + configs.PVCExt
			err = rsp.CreatePVC(sp, configs, pvcName)
			if err != nil {
				return
			}
			mountPath, err := populateMountPath(sp, configs)
			if err != nil {
				return err
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
		err = rsp.CreateConfigMap(sp, deployYAMLCMName, data)
		if err != nil {
			return
		}
		mountPath := configs.SiddhiHome + configs.DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = configs.DepConfParameter + mountPath + deployYAMLCMName
	} else {
		if sp.Spec.SiddhiConfig != "" {
			deployYAMLCMName := sp.Name + configs.DepCMExt
			data := map[string]string{
				deployYAMLCMName: sp.Spec.SiddhiConfig,
			}
			err = rsp.CreateConfigMap(sp, deployYAMLCMName, data)
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
	}

	configMapName := siddhiApp.Name + configs.SiddhiCMExt
	for k, v := range siddhiApp.Apps {
		key := k + configs.SiddhiExt
		configMapData[key] = v
	}
	err = rsp.CreateConfigMap(sp, configMapName, configMapData)
	if err != nil {
		return err
	}
	siddhiFilesPath := configs.SiddhiHome + configs.SiddhiFilesDir
	volume, volumeMount := createCMVolumes(configMapName, siddhiFilesPath)
	volumes = append(volumes, volume)
	volumeMounts = append(volumeMounts, volumeMount)
	siddhiFilesParameter := configs.AppConfParameter + siddhiFilesPath + " "
	userID := int64(802)
	err = rsp.CreateDeployment(
		sp,
		strings.ToLower(siddhiApp.Name),
		sp.Namespace,
		siddhiApp.Replicas,
		labels,
		siddhiRunnerImage,
		configs.ContainerName,
		[]string{configs.Shell},
		[]string{
			siddhiHome + configs.SiddhiBin + "/" + configs.SiddhiProfile + ".sh",
			siddhiFilesParameter,
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
func (rsp *ReconcileSiddhiProcess) updateErrorStatus(sp *siddhiv1alpha2.SiddhiProcess, eventRecorder record.EventRecorder, status Status, reason string, er error) *siddhiv1alpha2.SiddhiProcess {
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
	// err = rsp.client.Status().Update(context.TODO(), s)
	err := rsp.client.Status().Update(context.TODO(), sp)
	if err != nil {
		return s
	}
	return sp
}

// UpdateRunningStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Pending, Running
func (rsp *ReconcileSiddhiProcess) updateRunningStatus(sp *siddhiv1alpha2.SiddhiProcess, eventRecorder record.EventRecorder, status Status, reason string, message string) *siddhiv1alpha2.SiddhiProcess {
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
func (rsp *ReconcileSiddhiProcess) createArtifacts(sp *siddhiv1alpha2.SiddhiProcess, siddhiApps []SiddhiApp, configs Configs) *siddhiv1alpha2.SiddhiProcess {
	needDep := 0
	availableDep := 0
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	for _, siddhiApp := range siddhiApps {
		needDep++
		deployment := &appsv1.Deployment{}
		err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err != nil && apierrors.IsNotFound(err) {
			err = rsp.deployApp(sp, siddhiApp, ER, configs)
			if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "AppDeploymentError", err)
				continue
			}
			availableDep++
			sp = rsp.updateRunningStatus(sp, ER, RUNNING, "DeploymentCreated", (siddhiApp.Name + " deployment created successfully"))
		} else if err != nil {
			sp = rsp.updateErrorStatus(sp, ER, ERROR, "DeploymentNotFound", err)
			continue
		} else {
			availableDep++
		}

		if siddhiApp.ServiceEnabled {
			service := &corev1.Service{}
			err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: siddhiApp.Name, Namespace: sp.Namespace}, service)
			if err != nil && apierrors.IsNotFound(err) {
				err := rsp.CreateService(sp, siddhiApp, configs)
				if err != nil {
					sp = rsp.updateErrorStatus(sp, ER, WARNING, "ServiceCreationError", err)
					continue
				}
				sp = rsp.updateRunningStatus(sp, ER, RUNNING, "ServiceCreated", (siddhiApp.Name + " service created successfully"))
			} else if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "ServiceNotFound", err)
				continue
			}

			if configs.AutoCreateIngress {
				ingress := &extensionsv1beta1.Ingress{}
				err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.HostName, Namespace: sp.Namespace}, ingress)
				if err != nil && apierrors.IsNotFound(err) {
					err := rsp.CreateIngress(sp, siddhiApp, configs)
					if err != nil {
						sp = rsp.updateErrorStatus(sp, ER, ERROR, "IngressCreationError", err)
						continue
					}
					reqLogger.Info("New ingress created successfully", "Ingress.Name", configs.HostName)
				} else if err == nil {
					err := rsp.UpdateIngress(sp, ingress, siddhiApp, configs)
					if err != nil {
						sp = rsp.updateErrorStatus(sp, ER, ERROR, "IngressUpdationError", err)
						continue
					}
				}
			}
		}
	}
	sp = rsp.updateReady(sp, availableDep, needDep)
	return sp
}

// checkDeployments function check the availability of deployments and the replications of the deployments.
func (rsp *ReconcileSiddhiProcess) checkDeployments(sp *siddhiv1alpha2.SiddhiProcess, siddhiApps []SiddhiApp) *siddhiv1alpha2.SiddhiProcess {
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
func (rsp *ReconcileSiddhiProcess) populateSiddhiApps(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) (siddhiApps []SiddhiApp, err error) {
	if _, ok := SPContainer[sp.Name]; ok {
		siddhiApps = SPContainer[sp.Name]
	} else {
		siddhiApps, err = rsp.parseApp(sp, configs)
		if err != nil {
			return
		}
		SPContainer[sp.Name] = siddhiApps
	}
	return
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
