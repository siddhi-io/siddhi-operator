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
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	"gopkg.in/yaml.v2"
	"path/filepath"
)

// labelsForSiddhiProcess returns the labels for selecting the resources
// belonging to the given SiddhiProcess custom resource object.
func labelsForSiddhiProcess(appName string, configs Configs) map[string]string {
	return map[string]string{
		"siddhi.io/name":     configs.CRDName,
		"siddhi.io/instance": appName,
		"siddhi.io/version":  configs.OperatorVersion,
		"siddhi.io/part-of":  configs.OperatorName,
	}
}

// getStatus return relevant status to a given integer. This uses status array and the constants list.
func getStatus(n Status) string {
	return status[n]
}

// GetAppName return the app name for given siddhi app. This function used two regex to extract the name properly.
// Here used two regex for sigle quoted names and double quoted names.
func getAppName(app string) (appName string, err error) {
	re := regexp.MustCompile(".*@App:name\\(\"(.*)\"\\)\\s*\n")
	match := re.FindStringSubmatch(app)
	if len(match) >= 2 {
		appName = strings.TrimSpace(match[1])
		return
	}
	re = regexp.MustCompile(".*@App:name\\('(.*)'\\)\\s*\n")
	match = re.FindStringSubmatch(app)
	if len(match) >= 2 {
		appName = strings.TrimSpace(match[1])
		return
	}
	err = errors.New("Siddhi app name extraction error")
	return
}

// populateParserRequest creates the request which send to the siddhi parser during the runtime.
func populateParserRequest(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiApps []string,
	propertyMap map[string]string,
	configs Configs,
) (siddhiParserRequest SiddhiParserRequest) {
	siddhiParserRequest = SiddhiParserRequest{
		SiddhiApps:  siddhiApps,
		PropertyMap: propertyMap,
	}

	ms := siddhiv1alpha2.MessagingSystem{}
	if sp.Spec.MessagingSystem.TypeDefined() {
		if sp.Spec.MessagingSystem.EmptyConfig() {
			ms = siddhiv1alpha2.MessagingSystem{
				Type: configs.NATSMSType,
				Config: siddhiv1alpha2.MessagingConfig{
					ClusterID: configs.STANClusterName,
					BootstrapServers: []string{
						configs.NATSDefaultURL,
					},
				},
			}
		} else {
			ms = sp.Spec.MessagingSystem
		}
		siddhiParserRequest = SiddhiParserRequest{
			SiddhiApps:      siddhiApps,
			PropertyMap:     propertyMap,
			MessagingSystem: &ms,
		}
	}

	return
}

// invokeParser simply invoke the siddhi parser within the k8s cluster
func invokeParser(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiParserRequest SiddhiParserRequest,
	configs Configs,
) (siddhiAppConfigs []SiddhiAppConfig, err error) {
	url := configs.ParserDomain + sp.Namespace + configs.ParserContext
	b, err := json.Marshal(siddhiParserRequest)
	if err != nil {
		return
	}
	var jsonStr = []byte(string(b))
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New(url + " invalid HTTP response status " + strconv.Itoa(http.StatusOK))
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&siddhiAppConfigs)
	if err != nil {
		return
	}
	return
}

// populateRunnerConfigs sends the relevant information about the siddhi runner deployment
func populateRunnerConfigs(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) (image string, home string, secret string) {
	image = configs.SiddhiImage
	home = configs.SiddhiHome
	secret = sp.Spec.ImagePullSecret

	if sp.Spec.Container.Image != "" {
		image = sp.Spec.Container.Image
	}
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
func populateMountPath(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) (mountPath string, err error) {
	spConf := &SiddhiConfig{}
	err = yaml.Unmarshal([]byte(sp.Spec.SiddhiConfig), spConf)
	if err != nil {
		return
	}
	mountPath = configs.SiddhiHome + configs.FilePersistentPath
	if spConf.StatePersistence.SPConfig.Location != "" && filepath.IsAbs(spConf.StatePersistence.SPConfig.Location) {
		mountPath = spConf.StatePersistence.SPConfig.Location
	} else if spConf.StatePersistence.SPConfig.Location != "" {
		mountPath = configs.SiddhiHome + configs.SiddhiRunnerPath + spConf.StatePersistence.SPConfig.Location
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

// pathContains checks the given path is available on ingress path lists or not
func pathContains(paths []extensionsv1beta1.HTTPIngressPath, path extensionsv1beta1.HTTPIngressPath) bool {
	for _, p := range paths {
		if reflect.DeepEqual(p, path) {
			return true
		}
	}
	return false
}
