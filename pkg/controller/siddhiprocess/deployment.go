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
	"strings"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/record"
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
func (rsp *ReconcileSiddhiProcess) deployApp(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApp SiddhiApp,
	eventRecorder record.EventRecorder,
	configs Configs,
) (err error) {

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	configMapData := make(map[string]string)
	labels := labelsForSiddhiProcess(siddhiApp.Name, configs)
	siddhiRunnerImage, siddhiHome, siddhiImageSecret := populateRunnerConfigs(sp, configs)
	containerPorts := siddhiApp.ContainerPorts

	if siddhiImageSecret != "" {
		secret := createLocalObjectReference(siddhiImageSecret)
		imagePullSecrets = append(imagePullSecrets, secret)
	}

	q := siddhiv1alpha2.PV{}
	if !(sp.Spec.PV.Equals(&q)) && siddhiApp.PersistenceEnabled {
		pvcName := siddhiApp.Name + configs.PVCExt
		err = rsp.createPVC(sp, configs, pvcName)
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

	configMapName := siddhiApp.Name + configs.SiddhiCMExt
	for k, v := range siddhiApp.Apps {
		key := k + configs.SiddhiExt
		configMapData[key] = v
	}
	err = rsp.createConfigMap(sp, configMapName, configMapData)
	if err != nil {
		return err
	}
	mountPath := configs.SiddhiHome + configs.SiddhiFileRPath
	volume, volumeMount := createCMVolumes(configMapName, mountPath)
	volumes = append(volumes, volume)
	volumeMounts = append(volumeMounts, volumeMount)

	configParameter := ""
	if siddhiApp.PersistenceEnabled {
		deployYAMLCMName := sp.Name + configs.DepCMExt
		siddhiConfig := StatePersistenceConf
		if sp.Spec.SiddhiConfig != "" {
			siddhiConfig = sp.Spec.SiddhiConfig
		}
		data := map[string]string{
			deployYAMLCMName: siddhiConfig,
		}
		err = rsp.createConfigMap(sp, deployYAMLCMName, data)
		if err != nil {
			return
		}
		mountPath := siddhiHome + configs.DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = configs.DepConfParameter + siddhiHome + configs.DepConfMountPath + deployYAMLCMName
	}

	userID := int64(802)
	deployment := createDeployment(
		strings.ToLower(siddhiApp.Name),
		sp.Namespace,
		siddhiApp.Replicas,
		labels,
		siddhiRunnerImage,
		configs.ContainerName,
		[]string{configs.Shell},
		[]string{siddhiHome + configs.RunnerRPath, configParameter},
		containerPorts,
		volumeMounts,
		sp.Spec.Container.Env,
		corev1.SecurityContext{RunAsUser: &userID},
		corev1.PullAlways,
		imagePullSecrets,
		volumes,
	)
	controllerutil.SetControllerReference(sp, deployment, rsp.scheme)
	err = rsp.client.Create(context.TODO(), deployment)
	if err != nil {
		return
	}
	return
}

// todo rediness and liveness probes
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
