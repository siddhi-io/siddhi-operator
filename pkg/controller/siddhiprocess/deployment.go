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
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/record"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
// Inputs - SiddhiProcess object reference, siddhiApp object that holds the details of the deployment, default config object, and event recorder to record the events
// Process -
//     First gets the envs of the operator deployment if it defines and creates the docker image name
//     After that creates persistence volumen claim
//     Enable container ports
//     Creates config map for siddhi apps
//     Creates a config map for the YAML block which used to change the deployment.yaml of the siddhi-runner
//     Set envs
//     Set controller reference and returns
func (rsp *ReconcileSiddhiProcess) deployApp(
	sp *siddhiv1alpha1.SiddhiProcess,
	siddhiApp SiddhiApp,
	operatorEnvs map[string]string,
	configs Configs,
	eventRecorder record.EventRecorder,
) (*appsv1.Deployment, *siddhiv1alpha1.SiddhiProcess, error) {

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var enviromentVariables []corev1.EnvVar
	var containerPorts []corev1.ContainerPort
	var err error
	em := ""
	configMapData := make(map[string]string)
	replicas := siddhiApp.Replicas
	siddhiConfig := sp.Spec.SiddhiConfig
	deployYAMLCMName := sp.Name + configs.DepCMExt
	siddhiHome := configs.SiddhiHome
	siddhiRunnerImageName := configs.SiddhiRunnerImage
	siddhiRunnerImagetag := configs.SiddhiRunnerImageTag
	labels := labelsForSiddhiProcess(strings.ToLower(siddhiApp.Name), operatorEnvs, configs)

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
	if !(sp.Spec.DeploymentConfigs.PersistenceVolume.Equals(&q)) && siddhiApp.CreatePVC {
		pvcName := strings.ToLower(siddhiApp.Name + configs.PVCExt)
		err = rsp.createPVC(sp, configs, pvcName)
		if err != nil {
			em = "Failed to create new PVC : " + pvcName
			sp = rsp.updateStatus(ERROR, "PVCCreationError", em, ER, err, sp)
		} else {
			spConf := &SiddhiConfig{}
			err := yaml.Unmarshal([]byte(sp.Spec.SiddhiConfig), spConf)
			if err != nil {
				em = "Failed to marshal state.persistence YAML in " + sp.Name
				sp = rsp.updateStatus(ERROR, "YAMLMarshalError", em, ER, err, sp)
			}
			mountPath := siddhiHome + configs.FilePersistentPath
			if spConf.StatePersistence.SPConfig.Location != "" && filepath.IsAbs(spConf.StatePersistence.SPConfig.Location) {
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
		em = "Failed to create new ConfigMap : " + configMapName
		sp = rsp.updateStatus(ERROR, "CMCreationError", em, ER, err, sp)
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
	if siddhiConfig != "" && siddhiApp.Type != PassthroughApp {
		data := map[string]string{
			deployYAMLCMName: siddhiConfig,
		}
		err = rsp.createConfigMap(sp, deployYAMLCMName, data)
		if err != nil {
			em = "Failed to create new ConfigMap : " + deployYAMLCMName
			sp = rsp.updateStatus(ERROR, "CMCreationError", em, ER, err, sp)
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
	} else if siddhiConfig == "" && sp.Spec.DeploymentConfigs.Mode == Failover && siddhiApp.Type == ProcessApp {
		data := map[string]string{
			deployYAMLCMName: StatePersistenceConf,
		}
		err = rsp.createConfigMap(sp, deployYAMLCMName, data)
		if err != nil {
			em = "Failed to create new ConfigMap : " + deployYAMLCMName
			sp = rsp.updateStatus(ERROR, "CMCreationError", em, ER, err, sp)
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
	return deployment, sp, err
}

// createDeployment creates a deployment for given set of configuration data.
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
