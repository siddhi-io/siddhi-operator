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

package deploymanager

import (
	"strconv"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	artifact "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/artifact"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/intstr"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Application contains details about the siddhi app which need in the K8s deployment
type Application struct {
	Name               string                 `json:"appName"`
	ContainerPorts     []corev1.ContainerPort `json:"containerPorts"`
	Apps               map[string]string      `json:"apps"`
	ServiceEnabled     bool                   `json:"serviceEnabled"`
	PersistenceEnabled bool                   `json:"persistenceEnabled"`
	Replicas           int32                  `json:"replicas"`
}

// Image holds the docker image details of the deployment
type Image struct {
	Name    string
	Home    string
	Secret  string
	Profile string
}

// DeployManager creates and manage the deployment
type DeployManager struct {
	Application   Application
	KubeClient    artifact.KubeClient
	Labels        map[string]string
	Image         Image
	SiddhiProcess *siddhiv1alpha2.SiddhiProcess
}

// SPConfig contains the state persistence configs
type SPConfig struct {
	Location string `yaml:"location"`
}

// StatePersistence contains the StatePersistence config block
type StatePersistence struct {
	SPConfig SPConfig `yaml:"config"`
}

// Config contains the siddhi config block
type Config struct {
	StatePersistence StatePersistence `yaml:"statePersistence"`
}

// CreateLabels creates labels for each deployment
func (d *DeployManager) CreateLabels() {
	labels := map[string]string{
		"siddhi.io/name":     CRDName,
		"siddhi.io/instance": d.Application.Name,
		"siddhi.io/version":  OperatorVersion,
		"siddhi.io/part-of":  OperatorName,
	}
	d.Labels = labels
}

// Deploy creates a deployment according to the given Application specs.
// It create and mount volumes, create config maps, populates envs that needs to the deployment.
func (d *DeployManager) Deploy() (operationResult controllerutil.OperationResult, err error) {

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var imagePullSecrets []corev1.LocalObjectReference
	var depStrategy appsv1.DeploymentStrategy
	configParameter := ""
	appParameter := ""
	appsMap := make(map[string]string)

	d.CreateLabels()
	containerPorts := d.Application.ContainerPorts
	if d.Image.Secret != "" {
		secret := createLocalObjectReference(d.Image.Secret)
		imagePullSecrets = append(imagePullSecrets, secret)
	}

	q := corev1.PersistentVolumeClaimSpec{}
	if d.Application.PersistenceEnabled {
		if !siddhiv1alpha2.EqualsPVCSpec(&d.SiddhiProcess.Spec.PVC, &q) {
			pvcName := d.Application.Name + PVCExtension
			err = d.KubeClient.CreateOrUpdatePVC(
				pvcName,
				d.SiddhiProcess.Namespace,
				d.SiddhiProcess.Spec.PVC,
				d.SiddhiProcess,
			)
			if err != nil {
				return
			}
			mountPath := ""
			mountPath, err = populateMountPath(d.SiddhiProcess, d.Image.Home, d.Image.Profile)
			if err != nil {
				return
			}
			volume, volumeMount := createPVCVolumes(pvcName, mountPath)
			volumes = append(volumes, volume)
			volumeMounts = append(volumeMounts, volumeMount)
		}
		deployYAMLCMName := d.Application.Name + DepCMExtension
		siddhiConfig := StatePersistenceConf
		if d.SiddhiProcess.Spec.SiddhiConfig != "" {
			siddhiConfig = d.SiddhiProcess.Spec.SiddhiConfig
		}
		data := map[string]string{
			deployYAMLCMName: siddhiConfig,
		}
		err = d.KubeClient.CreateOrUpdateCM(deployYAMLCMName, d.SiddhiProcess.Namespace, data, d.SiddhiProcess)
		if err != nil {
			return
		}
		mountPath := d.Image.Home + DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = DepConfParameter + mountPath + deployYAMLCMName
	} else if d.SiddhiProcess.Spec.SiddhiConfig != "" {
		deployYAMLCMName := d.SiddhiProcess.Name + DepCMExtension
		data := map[string]string{
			deployYAMLCMName: d.SiddhiProcess.Spec.SiddhiConfig,
		}
		err = d.KubeClient.CreateOrUpdateCM(deployYAMLCMName, d.SiddhiProcess.Namespace, data, d.SiddhiProcess)
		if err != nil {
			return
		}
		mountPath := d.Image.Home + DepConfMountPath
		volume, volumeMount := createCMVolumes(deployYAMLCMName, mountPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		configParameter = DepConfParameter + mountPath + deployYAMLCMName
	}

	maxUnavailable := intstr.IntOrString{
		Type:   artifact.Int,
		IntVal: MaxUnavailable,
	}
	maxSurge := intstr.IntOrString{
		Type:   artifact.Int,
		IntVal: MaxSurge,
	}
	rollingUpdate := appsv1.RollingUpdateDeployment{
		MaxUnavailable: &maxUnavailable,
		MaxSurge:       &maxSurge,
	}
	depStrategy = appsv1.DeploymentStrategy{
		Type:          appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &rollingUpdate,
	}

	if len(d.Application.Apps) > 0 {
		appsCMName := d.Application.Name + strconv.Itoa(int(d.SiddhiProcess.Status.CurrentVersion))
		for k, v := range d.Application.Apps {
			key := k + SiddhiExtension
			appsMap[key] = v
		}
		err = d.KubeClient.CreateOrUpdateCM(appsCMName, d.SiddhiProcess.Namespace, appsMap, d.SiddhiProcess)
		if err != nil {
			return
		}
		appsPath := d.Image.Home + SiddhiFilesDir
		volume, volumeMount := createCMVolumes(appsCMName, appsPath)
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
		appParameter = AppConfParameter + appsPath + " "
	} else {
		appParameter = ParserParameter
	}

	userID := int64(802)
	operationResult, _ = d.KubeClient.CreateOrUpdateDeployment(
		d.Application.Name,
		d.SiddhiProcess.Namespace,
		d.Application.Replicas,
		d.Labels,
		d.Image.Name,
		ContainerName,
		[]string{Shell},
		[]string{
			d.Image.Home + SiddhiBin + "/" + d.Image.Profile + ".sh",
			appParameter,
			configParameter,
		},
		containerPorts,
		volumeMounts,
		d.SiddhiProcess.Spec.Container.Env,
		corev1.SecurityContext{RunAsUser: &userID},
		corev1.PullAlways,
		imagePullSecrets,
		volumes,
		depStrategy,
		d.SiddhiProcess,
	)
	return
}

// createLocalObjectReference creates a local object reference secret to download docker images from private registries.
func createLocalObjectReference(secret string) (localObjectRef corev1.LocalObjectReference) {
	localObjectRef = corev1.LocalObjectReference{
		Name: secret,
	}
	return
}

// populateMountPath reads the runner configs given by the user.
// Check whether the given path is absolute or not.
// If it is a absolute path then use that path to persist siddhi apps.
// Otherwise use relative path w.r.t default runner home
func populateMountPath(sp *siddhiv1alpha2.SiddhiProcess, home string, profile string) (mountPath string, err error) {
	config := &Config{}
	err = yaml.Unmarshal([]byte(sp.Spec.SiddhiConfig), config)
	if err != nil {
		return
	}
	mountPath = home + WSO2Dir + "/" + profile + "/" + FilePersistentDir
	if config.StatePersistence.SPConfig.Location != "" && filepath.IsAbs(config.StatePersistence.SPConfig.Location) {
		mountPath = config.StatePersistence.SPConfig.Location
	} else if config.StatePersistence.SPConfig.Location != "" {
		mountPath = home + WSO2Dir + "/" + profile + "/" + config.StatePersistence.SPConfig.Location
	}
	return
}

// createCMVolumes creates volume and volume mount for a config map
func createCMVolumes(configMapName string, mountPath string) (volume corev1.Volume, volumeMount corev1.VolumeMount) {
	volume = corev1.Volume{
		Name: configMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}
	volumeMount = corev1.VolumeMount{
		Name:      configMapName,
		MountPath: mountPath,
	}
	return
}

// createCMVolumes creates volume and volume mount for a PVC
func createPVCVolumes(pvcName string, mountPath string) (volume corev1.Volume, volumeMount corev1.VolumeMount) {
	volume = corev1.Volume{
		Name: pvcName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	volumeMount = corev1.VolumeMount{
		Name:      pvcName,
		MountPath: mountPath,
	}
	return
}
