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
	"strconv"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// SiddhiApp contains details about the siddhi app which need by K8s deployment
type SiddhiApp struct {
	Name               string                 `json:"appName"`
	ContainerPorts     []corev1.ContainerPort `json:"containerPorts"`
	Apps               map[string]string      `json:"apps"`
	ServiceEnabled     bool                   `json:"serviceEnabled"`
	PersistenceEnabled bool                   `json:"persistenceEnabled"`
	Replicas           int32                  `json:"replicas"`
}

// TemplatedApp contains the templated siddhi app and relevant properties to pass into the parser service
type TemplatedApp struct {
	App         string            `json:"siddhiApp"`
	PropertyMap map[string]string `json:"propertyMap"`
}

// SiddhiParserRequest is request struct of siddhi-parser
type SiddhiParserRequest struct {
	SiddhiApps      []string                       `json:"siddhiApps"`
	PropertyMap     map[string]string              `json:"propertyMap"`
	MessagingSystem siddhiv1alpha1.MessagingSystem `json:"messagingSystem"`
}

// SourceDeploymentConfig hold deployment configs of a particular siddhi app
type SourceDeploymentConfig struct {
	ServiceProtocol string `json:"serviceProtocol"`
	Secured         bool   `json:"secured"`
	Port            int    `json:"port"`
	IsPulling       bool   `json:"isPulling"`
}

// SourceList hold list of object which contains configurations of a siddhi app
type SourceList struct {
	SourceDeploymentConfigs []SourceDeploymentConfig `json:"sourceDeploymentConfigs"`
}

// SiddhiAppConfig holds siddhi app and the relevant SourceList
type SiddhiAppConfig struct {
	SiddhiApp          string     `json:"siddhiApp"`
	SiddhiSourceList   SourceList `json:"sourceList"`
	PersistenceEnabled bool       `json:"persistenceEnabled"`
	Replicas           int32      `json:"replicas"`
}

// SiddhiFAppConfig holds siddhi apps of the failover scenario and relevant SourceList
type SiddhiFAppConfig struct {
	PassthroughApp   string     `json:"passthroughApp"`
	QueryApp         string     `json:"queryApp"`
	SiddhiSourceList SourceList `json:"sourceList"`
}

// SiddhiParserResponse is the response object of siddhi-parser
type SiddhiParserResponse struct {
	AppConfig           []SiddhiAppConfig  `json:"siddhiAppConfigs"`
	FailoverDeployments []SiddhiFAppConfig `json:"failoverDeployments"`
}

// parseFailoverApp call MSF4J service and parse a given siddhiApp.
// Here parser call an endpoint according to the deployment type - default, failover, and distributed
// After that REST call, the siddhi parser returns relevant details of the deployment. This function get those details and
// encapsulate all the details into a common structure(SiddhiApp) regarless of the deployment type.
// Siddhi operator used this general SiddhiApp object to the further process.
func (rsp *ReconcileSiddhiProcess) parseApp(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) (siddhiAppStructs []SiddhiApp, err error) {
	apps := make(map[string]string)
	siddhiApps := rsp.getSiddhiApps(sp)
	propertyMap := rsp.populateUserEnvs(sp)
	siddhiParserRequest := populateParserRequest(sp, siddhiApps, propertyMap, configs)
	siddhiParserResponse, err := invokeParser(sp, siddhiParserRequest, configs)
	if err != nil {
		return
	}

	for i, siddhiDepConf := range siddhiParserResponse.AppConfig {
		var ports []corev1.ContainerPort
		var protocols []string
		var tls []bool
		serviceEnabled := false
		app := siddhiDepConf.SiddhiApp
		appName, err := GetAppName(app)
		deploymentName := sp.Name + "-" + strconv.Itoa(i)
		if err != nil {
			return siddhiAppStructs, err
		}
		for _, deploymentConf := range siddhiDepConf.SiddhiSourceList.SourceDeploymentConfigs {
			if !deploymentConf.IsPulling {
				serviceEnabled = true
				port := corev1.ContainerPort{
					Name:          deploymentName + "-" + strconv.Itoa(deploymentConf.Port),
					ContainerPort: int32(deploymentConf.Port),
					Protocol:      corev1.Protocol(deploymentConf.ServiceProtocol),
				}
				ports = append(ports, port)
				protocols = append(protocols, deploymentConf.ServiceProtocol)
				tls = append(tls, deploymentConf.Secured)
			}
		}
		if len(ports) > 0 {
			serviceEnabled = true
		}
		apps[appName] = app
		siddhiAppStruct := SiddhiApp{
			Name:               deploymentName,
			ContainerPorts:     ports,
			Apps:               apps,
			ServiceEnabled:     serviceEnabled,
			PersistenceEnabled: siddhiDepConf.PersistenceEnabled,
			Replicas:           siddhiDepConf.Replicas,
		}
		siddhiAppStructs = append(siddhiAppStructs, siddhiAppStruct)
	}

	return siddhiAppStructs, err
}
