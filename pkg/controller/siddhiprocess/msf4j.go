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

// SiddhiApp contains details about the siddhi app which need by K8s
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
	SiddhiApps  []string          `json:"siddhiApps"`
	PropertyMap map[string]string `json:"propertyMap"`
}

// SourceDeploymentConfig hold deployment configs
type SourceDeploymentConfig struct {
	ServiceProtocol string `json:"serviceProtocol"`
	Secured         bool   `json:"secured"`
	Port            int    `json:"port"`
	Strategy        string `json:"strategy"`
}

// SourceList hold source list object
type SourceList struct {
	SourceDeploymentConfigs []SourceDeploymentConfig `json:"sourceDeploymentConfigs"`
}

// SiddhiAppConfig holds siddhi app config details
type SiddhiAppConfig struct {
	SiddhiApp        string     `json:"siddhiApp"`
	SiddhiSourceList SourceList `json:"sourceList"`
}

// SiddhiParserResponse siddhi-parser response
type SiddhiParserResponse struct {
	AppConfig []SiddhiAppConfig `json:"siddhiAppConfigs"`
}

// parseFailoverApp call MSF4J service and parse a given siddhiApp
func (rsp *ReconcileSiddhiProcess) parseApp(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) (siddhiAppStruct SiddhiApp, err error) {
	var resp *http.Response
	var ports []int
	var protocols []string
	var tls []bool
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
		err = errors.New("CRD should only contain either query or app entry")
	} else {
		err = errors.New("CRD must have either query or app entry to deploy siddhi apps")
	}
	propertyMap := rsp.populateUserEnvs(sp)
	siddhiParserRequest := SiddhiParserRequest{
		SiddhiApps:  siddhiApps,
		PropertyMap: propertyMap,
	}
	var siddhiParserResponse SiddhiParserResponse
	b, err := json.Marshal(siddhiParserRequest)
	if err != nil {
		reqLogger.Error(err, "JSON marshal error in apps config maps")
		return siddhiAppStruct, err
	}
	var jsonStr = []byte(string(b))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		reqLogger.Error(err, "REST invoking error")
		return siddhiAppStruct, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&siddhiParserResponse)
	for _, siddhiApp := range siddhiParserResponse.AppConfig {
		app := siddhiApp.SiddhiApp
		appName := strings.TrimSpace(GetAppName(app))
		for _, deploymentConf := range siddhiApp.SiddhiSourceList.SourceDeploymentConfigs {
			if deploymentConf.Strategy == Push {
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
	siddhiAppStruct = SiddhiApp{
		Name:      strings.ToLower(sp.Name),
		Ports:     ports,
		Protocols: protocols,
		TLS:       tls,
		CreateSVC: createSVC,
		Apps:      apps,
	}
	return siddhiAppStruct, err
}

// isIn used to find element in a given slice
func isIn(slice []int, element int) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}
