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
	"strconv"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

// parseFailoverApp call MSF4J service and parse a given siddhiApp.
// Here parser call an endpoint according to the deployment type - default, failover, and distributed
// After that REST call, the siddhi parser returns relevant details of the deployment. This function get those details and
// encapsulate all the details into a common structure(SiddhiApp) regarless of the deployment type.
// Siddhi operator used this general SiddhiApp object to the further process.
func (rsp *ReconcileSiddhiProcess) parseApp(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) (siddhiAppStructs []SiddhiApp, err error) {
	siddhiApps, err := rsp.getSiddhiApps(sp)
	if err != nil {
		return
	}
	propertyMap := rsp.populateUserEnvs(sp)
	siddhiParserRequest := populateParserRequest(sp, siddhiApps, propertyMap, configs)
	err = rsp.deployParser(sp, configs)
	siddhiAppConfigs, err := invokeParser(sp, siddhiParserRequest, configs)
	if err != nil {
		return
	}

	for i, siddhiDepConf := range siddhiAppConfigs {
		var ports []corev1.ContainerPort
		var protocols []string
		var tls []bool
		apps := make(map[string]string)
		serviceEnabled := false
		app := siddhiDepConf.SiddhiApp
		appName, err := getAppName(app)
		deploymentName := sp.Name + "-" + strconv.Itoa(i)
		if err != nil {
			return siddhiAppStructs, err
		}
		for _, deploymentConf := range siddhiDepConf.SourceDeploymentConfigs {
			if !deploymentConf.IsPulling {
				serviceEnabled = true
				port := corev1.ContainerPort{
					Name:          "p" + strconv.Itoa(deploymentConf.Port),
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
	err = rsp.cleanParser(sp)
	if err != nil {
		return
	}
	return siddhiAppStructs, err
}

// invokeParser simply invoke the siddhi parser within the k8s cluster
func invokeParser(
	sp *siddhiv1alpha2.SiddhiProcess,
	siddhiParserRequest SiddhiParserRequest,
	configs Configs,
) (siddhiAppConfigs []SiddhiAppConfig, err error) {
	url := configs.ParserHTTP + sp.Name + "." + sp.Namespace + configs.ParserContext
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
		err = errors.New(url + " invalid HTTP response status " + strconv.Itoa(resp.StatusCode))
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&siddhiAppConfigs)
	if err != nil {
		return
	}
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

func waitForParser(url string) (err error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryWaitMin = time.Duration(ParserMinWait) * time.Second
	retryClient.RetryWaitMax = time.Duration(ParserMaxWait) * time.Second
	retryClient.RetryMax = ParserMaxRetry
	retryClient.Logger = nil
	_, err = retryClient.Get(url)
	if err == nil {
		return
	}
	return
}
