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
	"regexp"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"gopkg.in/yaml.v2"
	"path/filepath"
)

// SPConfig contains the state persistence configs
type SPConfig struct {
	Location string `yaml:"location"`
}

// StatePersistence contains the StatePersistence config block
type StatePersistence struct {
	SPConfig SPConfig `yaml:"config"`
}

// SiddhiConfig contains the siddhi config block
type SiddhiConfig struct {
	StatePersistence StatePersistence `yaml:"state.persistence"`
}

// deployApp returns a sp Deployment object
func (rsp *ReconcileSiddhiProcess) deployApp(sp *siddhiv1alpha1.SiddhiProcess, siddhiApp SiddhiApp, operatorEnvs map[string]string, configs Configs) (*appsv1.Deployment, error) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var enviromentVariables []corev1.EnvVar
	var containerPorts []corev1.ContainerPort
	var err error
	configMapData := make(map[string]string)
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	replicas := int32(1)
	siddhiConfig := sp.Spec.SiddhiConfig
	deployYAMLCMName := sp.Name + "-deployment.yaml"
	siddhiHome := configs.SiddhiHome
	siddhiRunnerImageName := configs.SiddhiRunnerImage
	siddhiRunnerImagetag := configs.SiddhiRunnerImageTag
	labels := labelsForSiddhiProcess(sp.Name, operatorEnvs, configs)

	if operatorEnvs["SIDDHI_RUNNER_HOME"] != "" {
		siddhiHome = strings.TrimSpace(operatorEnvs["SIDDHI_RUNNER_HOME"])
	}
	if operatorEnvs["SIDDHI_RUNNER_IMAGE"] != "" {
		siddhiRunnerImageName = strings.TrimSpace(operatorEnvs["SIDDHI_RUNNER_IMAGE"])
	}
	if operatorEnvs["SIDDHI_RUNNER_IMAGE_TAG"] != "" {
		siddhiRunnerImagetag = strings.TrimSpace(operatorEnvs["SIDDHI_RUNNER_IMAGE_TAG"])
	}
	siddhiRunnerImage := siddhiRunnerImageName + ":" + siddhiRunnerImagetag
	if operatorEnvs["SIDDHI_RUNNER_IMAGE_SECRET"] != "" {
		siddhiRunnerImageSecret := strings.TrimSpace(operatorEnvs["SIDDHI_RUNNER_IMAGE_SECRET"])
		secret := corev1.LocalObjectReference{
			Name: siddhiRunnerImageSecret,
		}
		imagePullSecrets = append(imagePullSecrets, secret)
	}

	if (sp.Spec.SiddhiPod.Image != "") && (sp.Spec.SiddhiPod.ImageTag != "") {
		siddhiRunnerImageName = strings.TrimSpace(sp.Spec.SiddhiPod.Image)
		siddhiRunnerImagetag = strings.TrimSpace(sp.Spec.SiddhiPod.ImageTag)
		siddhiRunnerImage = siddhiRunnerImageName + ":" + siddhiRunnerImagetag
		if sp.Spec.SiddhiPod.ImagePullSecret != "" {
			siddhiRunnerImageSecret := strings.TrimSpace(sp.Spec.SiddhiPod.ImagePullSecret)
			secret := corev1.LocalObjectReference{
				Name: siddhiRunnerImageSecret,
			}
			imagePullSecrets = append(imagePullSecrets, secret)
		}
	}
	q := siddhiv1alpha1.PersistenceVolume{}
	if !(sp.Spec.DeploymentConfigs.PersistenceVolume.Equals(&q)) {
		pvcName := strings.ToLower(siddhiApp.Name + configs.PVCExt)
		err = rsp.createPVC(sp, configs, pvcName)
		if err != nil {
			reqLogger.Error(err, "Failed to create new PVC", "PVC.Namespace", sp.Namespace, "PVC.Name", sp.Name)
		} else {
			spConf := &SiddhiConfig{}
			err := yaml.Unmarshal([]byte(sp.Spec.SiddhiConfig), spConf)
			if err != nil {
				reqLogger.Error(err, "Failed to marshal state.persistence YAML", "PVC.Namespace", sp.Namespace, "PVC.Name", sp.Name)
			}
			mountPath := siddhiHome + configs.FilePersistentPath
			if spConf.StatePersistence.SPConfig.Location != "" && filepath.IsAbs(spConf.StatePersistence.SPConfig.Location){
				 mountPath = spConf.StatePersistence.SPConfig.Location
			} else if spConf.StatePersistence.SPConfig.Location != "" {
				mountPath = siddhiHome + configs.SiddhiRunnerPath + spConf.StatePersistence.SPConfig.Location
			}
			volume := corev1.Volume{
				Name: pvcName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			}
			volumes = append(volumes, volume)
			volumeMount := corev1.VolumeMount{
				Name:      pvcName,
				MountPath: mountPath,
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}
	}

	for _, port := range siddhiApp.Ports {
		containerPort := corev1.ContainerPort{
			ContainerPort: int32(port),
		}
		containerPorts = append(containerPorts, containerPort)
	}

	configMapName := strings.ToLower(siddhiApp.Name) + configs.SiddhiCMExt
	for k, v := range siddhiApp.Apps {
		key := k + configs.SiddhiExt
		configMapData[key] = v
	}
	err = rsp.createConfigMap(sp, configMapName, configMapData)
	if err != nil {
		reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", sp.Namespace, "ConfigMap.Name", configMapName)
	} else {
		volume := corev1.Volume{
			Name: configMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		}
		volumes = append(volumes, volume)
		volumeMount := corev1.VolumeMount{
			Name:      configMapName,
			MountPath: siddhiHome + configs.SiddhiFileRPath,
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}

	configParameter := ""
	if siddhiConfig != "" {
		data := map[string]string{
			deployYAMLCMName: siddhiConfig,
		}
		err = rsp.createConfigMap(sp, deployYAMLCMName, data)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", sp.Namespace, "ConfigMap.Name", deployYAMLCMName)
		} else {
			volume := corev1.Volume{
				Name: configs.DepConfigName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: deployYAMLCMName,
						},
					},
				},
			}
			volumes = append(volumes, volume)

			volumeMount := corev1.VolumeMount{
				Name:      configs.DepConfigName,
				MountPath: siddhiHome + configs.DepConfMountPath,
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}
		configParameter = configs.DepConfParameter + siddhiHome + configs.DepConfMountPath + deployYAMLCMName
	}

	if len(sp.Spec.EnviromentVariables) > 0 {
		for _, enviromentVariable := range sp.Spec.EnviromentVariables {
			env := corev1.EnvVar{
				Name:  enviromentVariable.Name,
				Value: enviromentVariable.Value,
			}
			enviromentVariables = append(enviromentVariables, env)
		}
	}

	userID := int64(802)
	deployment := createDeployment(
		strings.ToLower(siddhiApp.Name),
		sp.Namespace,
		replicas,
		labels,
		siddhiRunnerImage,
		configs.ContainerName,
		[]string{configs.Shell},
		[]string{siddhiHome + configs.RunnerRPath, configParameter},
		containerPorts,
		volumeMounts,
		enviromentVariables,
		corev1.SecurityContext{RunAsUser: &userID},
		corev1.PullAlways,
		imagePullSecrets,
		volumes,
	)
	controllerutil.SetControllerReference(sp, deployment, rsp.scheme)
	return deployment, err
}

// createDeployment creates a deployment
func createDeployment(
	name string,
	namespace string,
	replicas int32,
	labels map[string]string,
	image string,
	containerName string,
	command []string,
	args []string,
	ports []corev1.ContainerPort,
	vms []corev1.VolumeMount,
	envs []corev1.EnvVar,
	sc corev1.SecurityContext,
	ipp corev1.PullPolicy,
	secrets []corev1.LocalObjectReference,
	volumes []corev1.Volume,
) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            containerName,
							Command:         command,
							Args:            args,
							Ports:           ports,
							VolumeMounts:    vms,
							Env:             envs,
							SecurityContext: &sc,
							ImagePullPolicy: ipp,
						},
					},
					ImagePullSecrets: secrets,
					Volumes:          volumes,
				},
			},
		},
	}
	return deployment
}

// GetAppName return the app name for given siddhiAPP
func GetAppName(app string) (appName string) {
	re := regexp.MustCompile(".*@App:name\\(\"(.*)\"\\)")
	match := re.FindStringSubmatch(app)
	appName = match[1]
	return appName
}
