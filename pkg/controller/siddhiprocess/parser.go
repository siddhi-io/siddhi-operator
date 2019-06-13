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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
)

// SiddhiApp contains details about the siddhi app which need by K8s deployment
type SiddhiApp struct {
	Name      string            `json:"appName"`
	Ports     []int             `json:"ports"`
	Protocols []string          `json:"protocols"`
	TLS       []bool            `json:"tls"`
	Apps      map[string]string `json:"apps"`
	CreateSVC bool              `json:"createSVC"`
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
	SiddhiApp        string     `json:"siddhiApp"`
	SiddhiSourceList SourceList `json:"sourceList"`
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

// parseFailoverApp call MSF4J service and parse a given siddhiApp - default and failover
func (rsp *ReconcileSiddhiProcess) parseApp(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) (siddhiAppStructs []SiddhiApp, err error) {
	var resp *http.Response
	var siddhiApps []string
	var url string
	apps := make(map[string]string)
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	query := sp.Spec.Query
	createSVC := false
	if sp.Spec.DeploymentConfigs.Mode == Failover {
		url = configs.ParserDomain + sp.Namespace + configs.ParserNATSContext
	} else {
		url = configs.ParserDomain + sp.Namespace + configs.ParserDefaultContext
	}

	if (query == "") && (len(sp.Spec.Apps) > 0) {
		for _, siddhiCMName := range sp.Spec.Apps {
			configMap := &corev1.ConfigMap{}
			rsp.client.Get(context.TODO(), types.NamespacedName{Name: siddhiCMName, Namespace: sp.Namespace}, configMap)
			for _, siddhiFileContent := range configMap.Data {
				siddhiApps = append(siddhiApps, siddhiFileContent)
			}
		}
	} else if (query != "") && (len(sp.Spec.Apps) <= 0) {
		siddhiApps = append(siddhiApps, query)
	} else if (query != "") && (len(sp.Spec.Apps) > 0) {
		err = errors.New("Custom resource should only contain either query or app entry")
	} else {
		err = errors.New("Custom resource must have either query or app entry to deploy siddhi apps")
	}

	propertyMap := rsp.populateUserEnvs(sp)
	siddhiParserRequest := SiddhiParserRequest{
		SiddhiApps:  siddhiApps,
		PropertyMap: propertyMap,
	}
	if sp.Spec.DeploymentConfigs.Mode == Failover {
		ms := siddhiv1alpha1.MessagingSystem{}
		if sp.Spec.DeploymentConfigs.MessagingSystem.Equals(&ms) {
			ms = siddhiv1alpha1.MessagingSystem{
				Type: configs.NATSMSType,
				Config: siddhiv1alpha1.MessagingSystemConfig{
					ClusterID: configs.NATSClusterName,
					BootstrapServers: []string{
						configs.NATSDefaultURL,
					},
				},
			}
		} else {
			ms = sp.Spec.DeploymentConfigs.MessagingSystem
		}
		siddhiParserRequest = SiddhiParserRequest{
			SiddhiApps:      siddhiApps,
			PropertyMap:     propertyMap,
			MessagingSystem: ms,
		}
	}

	var siddhiParserResponse SiddhiParserResponse
	b, err := json.Marshal(siddhiParserRequest)
	if err != nil {
		reqLogger.Error(err, ("JSON marshal error in SiddhiProcess : " + sp.Name))
		return siddhiAppStructs, err
	}
	var jsonStr = []byte(string(b))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		reqLogger.Error(err, ("Siddhi parser invoking error in URL : " + url))
		return siddhiAppStructs, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		err = errors.New("Siddhi parser invalid response from : " + url + ". Status : " + resp.Status)
		return siddhiAppStructs, errors.New(resp.Status)
	}
	json.NewDecoder(resp.Body).Decode(&siddhiParserResponse)
	if sp.Spec.DeploymentConfigs.Mode == Failover {
		for _, fApp := range siddhiParserResponse.FailoverDeployments {
			var ports []int
			var protocols []string
			var tls []bool
			for _, deploymentConf := range fApp.SiddhiSourceList.SourceDeploymentConfigs {
				if !deploymentConf.IsPulling {
					createSVC = true
					ports = append(ports, deploymentConf.Port)
					protocols = append(protocols, deploymentConf.ServiceProtocol)
					tls = append(tls, deploymentConf.Secured)
				}
			}
			if fApp.PassthroughApp != "" {
				pApp := fApp.PassthroughApp
				pAppName, err := GetAppName(pApp)
				if err != nil {
					return siddhiAppStructs, err
				}
				pAppStruct := SiddhiApp{
					Name:      strings.ToLower(pAppName),
					Ports:     ports,
					Protocols: protocols,
					TLS:       tls,
					CreateSVC: createSVC,
					Apps: map[string]string{
						pAppName: pApp,
					},
				}
				siddhiAppStructs = append(siddhiAppStructs, pAppStruct)
			}
			if fApp.QueryApp != "" {
				qApp := fApp.QueryApp
				qAppName, err := GetAppName(qApp)
				if err != nil {
					return siddhiAppStructs, err
				}
				qAppStruct := SiddhiApp{
					Name:      strings.ToLower(qAppName),
					CreateSVC: false,
					Apps: map[string]string{
						qAppName: qApp,
					},
				}
				siddhiAppStructs = append(siddhiAppStructs, qAppStruct)
			}
		}
	} else {
		var ports []int
		var protocols []string
		var tls []bool
		for _, siddhiApp := range siddhiParserResponse.AppConfig {
			app := siddhiApp.SiddhiApp
			appName, err := GetAppName(app)
			if err != nil {
				return siddhiAppStructs, err
			}
			for _, deploymentConf := range siddhiApp.SiddhiSourceList.SourceDeploymentConfigs {
				if !deploymentConf.IsPulling {
					createSVC = true
					ports = append(ports, deploymentConf.Port)
					protocols = append(protocols, deploymentConf.ServiceProtocol)
					tls = append(tls, deploymentConf.Secured)
				}
			}
			apps[appName] = app
		}
		if len(ports) > 0 {
			createSVC = true
		}
		siddhiAppStruct := SiddhiApp{
			Name:      strings.ToLower(sp.Name),
			Ports:     ports,
			Protocols: protocols,
			TLS:       tls,
			CreateSVC: createSVC,
			Apps:      apps,
		}
		siddhiAppStructs = append(siddhiAppStructs, siddhiAppStruct)
	}

	return siddhiAppStructs, err
}
