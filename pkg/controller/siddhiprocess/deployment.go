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

 import(
	 "regexp"
	 "strings"
	 "context"
	 "errors"
 
	 corev1 "k8s.io/api/core/v1"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 appsv1 "k8s.io/api/apps/v1"
	 siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
 
	 "k8s.io/apimachinery/pkg/types"
	 "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
 )
 
 // deploymentForMSiddhiProcess returns a siddhiProcess Deployment object
 func (reconcileSiddhiProcess *ReconcileSiddhiProcess) deploymentForSiddhiProcess(siddhiProcess *siddhiv1alpha1.SiddhiProcess, siddhiApp SiddhiApp, operatorEnvs map[string]string) (*appsv1.Deployment, error) {
	 labels := labelsForSiddhiProcess(siddhiProcess.Name, operatorEnvs)
	 reqLogger := log.WithValues("Request.Namespace", siddhiProcess.Namespace, "Request.Name", siddhiProcess.Name)
	 replicas := int32(1)
	 query := siddhiProcess.Spec.Query
	 siddhiConfig := siddhiProcess.Spec.SiddhiConfig
	 deploymentYAMLConfigMapName := siddhiProcess.Name + "-deployment.yaml"
	 siddhiHome := "/home/siddhi_user/siddhi-runner-0.1.0/"
	 siddhiRunnerImageName := "siddhiio/siddhi-runner-alpine"
	 siddhiRunnerImagetag := "v0.1.0"
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
	 if (siddhiProcess.Spec.SiddhiPod.Image != "") && (siddhiProcess.Spec.SiddhiPod.ImageTag != "") {
		 siddhiRunnerImageName = strings.TrimSpace(siddhiProcess.Spec.SiddhiPod.Image)
		 siddhiRunnerImagetag = strings.TrimSpace(siddhiProcess.Spec.SiddhiPod.ImageTag)
		 siddhiRunnerImage = siddhiRunnerImageName + ":" + siddhiRunnerImagetag
		 if siddhiProcess.Spec.SiddhiPod.ImagePullSecret != "" {
			 siddhiRunnerImageSecret := strings.TrimSpace(siddhiProcess.Spec.SiddhiPod.ImagePullSecret)
			 secret := corev1.LocalObjectReference{
				 Name: siddhiRunnerImageSecret,
			 }
			 imagePullSecrets = append(imagePullSecrets, secret)
		 }
	 }
	 for _, port := range siddhiApp.Ports{
		 containerPort := corev1.ContainerPort{
			 ContainerPort: int32(port),
		 }
		 containerPorts = append(containerPorts, containerPort)
	 }
	 if  (query == "") && (len(siddhiProcess.Spec.Apps) > 0) {
		 
		 configMap := &corev1.ConfigMap{}
		 configMapName := strings.ToLower(siddhiProcess.Name) + "-siddhi"
		 reconcileSiddhiProcess.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: siddhiProcess.Namespace}, configMap)
		 volume := corev1.Volume {
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
		 for siddhiFileName := range configMap.Data{
			 volumeMount := corev1.VolumeMount{
				 Name: configMapName,
				 MountPath: siddhiHome + "wso2/runner/deployment/siddhi-files/" + siddhiFileName,
				 SubPath:  siddhiFileName,
			 }
			 volumeMounts = append(volumeMounts, volumeMount)
		 }
		 
	 } else if (query != "") && (len(siddhiProcess.Spec.Apps) <= 0){
		 query = strings.TrimSpace(query)
		 appName := getAppName(query)
		 configMapName := strings.ToLower(siddhiProcess.Name) + "-siddhi"
		 configMap := reconcileSiddhiProcess.configMapForSiddhiApp(siddhiProcess, query, configMapName)
		 reqLogger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		 err := reconcileSiddhiProcess.client.Create(context.TODO(), configMap)
		 if err != nil {
			 reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		 } else{
			 volume := corev1.Volume {
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
				 Name: configMapName,
				 MountPath: siddhiHome + "wso2/runner/deployment/siddhi-files/" + appName + ".siddhi",
				 SubPath:  appName + ".siddhi",
			 }
			 volumeMounts = append(volumeMounts, volumeMount)
		 }
	 } else if (query != "") && (len(siddhiProcess.Spec.Apps) > 0){
		 err = errors.New("CRD should only contain either query or app entry")
	 } else {
		 err = errors.New("CRD must have either query or app entry to deploy siddhi apps")
	 }
	 
	 configParameter := ""
	 if siddhiConfig != ""{
		 configMap := reconcileSiddhiProcess.configMapForDeploymentYAML(siddhiProcess, siddhiConfig, deploymentYAMLConfigMapName)
		 reqLogger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		 err := reconcileSiddhiProcess.client.Create(context.TODO(), configMap)
		 if err != nil {
			 reqLogger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		 } else{
			 volume := corev1.Volume {
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
				 Name: "deploymentconfig",
				 MountPath: siddhiHome + "tmp/configs",
			 }
			 volumeMounts = append(volumeMounts, volumeMount)
		 }
		 configParameter = "-Dconfig="  + siddhiHome + "tmp/configs/" + deploymentYAMLConfigMapName
	 }
 
	 if len(siddhiProcess.Spec.EnviromentVariables) > 0 {
		 for _, enviromentVariable := range siddhiProcess.Spec.EnviromentVariables {
			 env := corev1.EnvVar{
				 Name: enviromentVariable.Name,
				 Value: enviromentVariable.Value,
			 }
			 enviromentVariables = append(enviromentVariables, env)
		 }
	 }
	 userID := int64(802)
	 sidddhiDeployment = &appsv1.Deployment{
		 TypeMeta: metav1.TypeMeta{
			 APIVersion: "apps/v1",
			 Kind:       "Deployment",
		 },
		 ObjectMeta: metav1.ObjectMeta{
			 Name:      siddhiProcess.Name,
			 Namespace: siddhiProcess.Namespace,
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
							 Ports: containerPorts,
							 VolumeMounts: volumeMounts,
							 Env: enviromentVariables,
							 SecurityContext: &corev1.SecurityContext{
								 RunAsUser: &userID,
							 },
							 ImagePullPolicy: corev1.PullAlways,
						 },
					 },
					 ImagePullSecrets: imagePullSecrets,
					 Volumes: volumes,
				 },
			 },
		 },
	 }
	 controllerutil.SetControllerReference(siddhiProcess, sidddhiDeployment, reconcileSiddhiProcess.scheme)
	 return sidddhiDeployment, err
 }
 
 // configMapForSiddhiApp returns a config map for the query string specified by the user in CRD
 func (reconcileSiddhiProcess *ReconcileSiddhiProcess) configMapForSiddhiApp(siddhiProcess *siddhiv1alpha1.SiddhiProcess, query string, appName string) *corev1.ConfigMap {
	 configMapKey := getAppName(query) + ".siddhi"
	 configMap := &corev1.ConfigMap{
		 TypeMeta: metav1.TypeMeta{
			 APIVersion: "v1",
			 Kind:       "ConfigMap",
		 },
		 ObjectMeta: metav1.ObjectMeta{
			 Name:      appName,
			 Namespace: siddhiProcess.Namespace,
		 },
		 Data: map[string]string{
			 configMapKey: query,
		 },
	 }
	 controllerutil.SetControllerReference(siddhiProcess, configMap, reconcileSiddhiProcess.scheme)
	 return configMap
 }
 
 // configMapForDeploymentYAML returns a config map for change deployment.yaml inside the siddhi-runner
 func (reconcileSiddhiProcess *ReconcileSiddhiProcess) configMapForDeploymentYAML(siddhiProcess *siddhiv1alpha1.SiddhiProcess, config string, configName string) *corev1.ConfigMap {
	 
	 configMap := &corev1.ConfigMap{
		 TypeMeta: metav1.TypeMeta{
			 APIVersion: "v1",
			 Kind:       "ConfigMap",
		 },
		 ObjectMeta: metav1.ObjectMeta{
			 Name:      configName,
			 Namespace: siddhiProcess.Namespace,
		 },
		 Data: map[string]string{
			 configName: config,
		 },
	 }
	 controllerutil.SetControllerReference(siddhiProcess, configMap, reconcileSiddhiProcess.scheme)
	 return configMap
 }
 
 // getAppName return the app name for given siddhiAPP
 func getAppName(app string) (appName string){
	 re := regexp.MustCompile(".*@App:name\\(\"(.*)\"\\)")
	 match := re.FindStringSubmatch(app)
	 appName = match[1]
	 return appName
 }