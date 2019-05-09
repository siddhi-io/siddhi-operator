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
	"errors"
	"regexp"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// deploymentForMSiddhiProcess returns a sp Deployment object
func (rsp *ReconcileSiddhiProcess) deploymentForSiddhiProcess(sp *siddhiv1alpha1.SiddhiProcess, siddhiApp SiddhiApp, operatorEnvs map[string]string, configs Configs) (*appsv1.Deployment, error) {
	labels := labelsForSiddhiProcess(sp.Name, operatorEnvs, configs)
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	replicas := int32(1)
	query := sp.Spec.Query
	siddhiConfig := sp.Spec.SiddhiConfig
	deploymentYAMLConfigMapName := sp.Name + "-deployment.yaml"
	siddhiHome := configs.SiddhiHome
	siddhiRunnerImageName := configs.SiddhiRunnerImage
	siddhiRunnerImagetag := configs.SiddhiRunnerImageTag
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var enviromentVariables []corev1.EnvVar
	var containerPorts []corev1.ContainerPort
	var err error
	var sidddhiDeployment *appsv1.Deployment

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
	for _, port := range siddhiApp.Ports {
		containerPort := corev1.ContainerPort{
			ContainerPort: int32(port),
		}
		containerPorts = append(containerPorts, containerPort)
	}
	if (query == "") && (len(sp.Spec.Apps) > 0) {

		configMap := &corev1.ConfigMap{}
		configMapName := strings.ToLower(sp.Name) + "-siddhi"
		rsp.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: sp.Namespace}, configMap)
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
		for siddhiFileName := range configMap.Data {
			volumeMount := corev1.VolumeMount{
				Name:      configMapName,
				MountPath: siddhiHome + "wso2/runner/deployment/siddhi-files/" + siddhiFileName,
				SubPath:   siddhiFileName,
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}

	} else if (query != "") && (len(sp.Spec.Apps) <= 0) {
		query = strings.TrimSpace(query)
		appName := GetAppName(query)
		configMapName := strings.ToLower(sp.Name) + "-siddhi"
		configMapKey := GetAppName(query) + ".siddhi"
		data := map[string]string{
			configMapKey: query,
		}
		configMap := rsp.createConfigMap(sp, configMapName, data)
		reqLogger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		err := rsp.client.Create(context.TODO(), configMap)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
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
				MountPath: siddhiHome + "wso2/runner/deployment/siddhi-files/" + appName + ".siddhi",
				SubPath:   appName + ".siddhi",
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}
	} else if (query != "") && (len(sp.Spec.Apps) > 0) {
		err = errors.New("CRD should only contain either query or app entry")
	} else {
		err = errors.New("CRD must have either query or app entry to deploy siddhi apps")
	}

	configParameter := ""
	if siddhiConfig != "" {
		data := map[string]string{
			deploymentYAMLConfigMapName: siddhiConfig,
		}
		configMap := rsp.createConfigMap(sp, deploymentYAMLConfigMapName, data)
		reqLogger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		err := rsp.client.Create(context.TODO(), configMap)
		if err != nil {
			reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		} else {
			volume := corev1.Volume{
				Name: "deploymentconfig",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: deploymentYAMLConfigMapName,
						},
					},
				},
			}
			volumes = append(volumes, volume)

			volumeMount := corev1.VolumeMount{
				Name:      "deploymentconfig",
				MountPath: siddhiHome + "tmp/configs",
			}
			volumeMounts = append(volumeMounts, volumeMount)
		}
		configParameter = "-Dconfig=" + siddhiHome + "tmp/configs/" + deploymentYAMLConfigMapName
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
	sidddhiDeployment = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sp.Name,
			Namespace: sp.Namespace,
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
							Image: siddhiRunnerImage,
							Name:  "siddhirunner-runtime",
							Command: []string{
								"sh",
							},
							Args: []string{
								siddhiHome + "bin/runner.sh",
								configParameter,
							},
							Ports:        containerPorts,
							VolumeMounts: volumeMounts,
							Env:          enviromentVariables,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &userID,
							},
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					ImagePullSecrets: imagePullSecrets,
					Volumes:          volumes,
				},
			},
		},
	}
	controllerutil.SetControllerReference(sp, sidddhiDeployment, rsp.scheme)
	return sidddhiDeployment, err
}

// GetAppName return the app name for given siddhiAPP
func GetAppName(app string) (appName string) {
	re := regexp.MustCompile(".*@App:name\\(\"(.*)\"\\)")
	match := re.FindStringSubmatch(app)
	appName = match[1]
	return appName
}
