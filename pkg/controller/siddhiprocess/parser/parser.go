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

package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	logr "github.com/go-logr/logr"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	artifact "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/artifact"
	deploymanager "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/deploymanager"
	corev1 "k8s.io/api/core/v1"
)

// Parser parse the application using parser deployment
type Parser struct {
	Name            string
	Namespace       string
	Apps            []string
	Env             map[string]string
	MessagingSystem siddhiv1alpha2.MessagingSystem
	Request         Request
	Logger          logr.Logger
	KubeClient      artifact.KubeClient
	Image           deploymanager.Image
	SiddhiProcess   *siddhiv1alpha2.SiddhiProcess
}

// Request is request struct of the siddhi-parser
type Request struct {
	Apps            []string                        `json:"siddhiApps"`
	PropertyMap     map[string]string               `json:"propertyMap"`
	MessagingSystem *siddhiv1alpha2.MessagingSystem `json:"messagingSystem,omitempty"`
}

// DeploymentConfig hold deployment configs of a particular siddhi app
type DeploymentConfig struct {
	ServiceProtocol string `json:"serviceProtocol"`
	Secured         bool   `json:"secured"`
	Port            int    `json:"port"`
	IsPulling       bool   `json:"isPulling"`
}

// AppConfig holds response of the parser
type AppConfig struct {
	SiddhiApp          string             `json:"siddhiApp"`
	DeploymentConfigs  []DeploymentConfig `json:"sourceDeploymentConfigs"`
	PersistenceEnabled bool               `json:"persistenceEnabled"`
	Replicas           int32              `json:"replicas"`
}

// Parse call MSF4J service and parse a given siddhiApp.
// Here parser call an endpoint according to the deployment type - default, failover, and distributed
// After that REST call, the siddhi parser returns relevant details of the deployment. This function get those details and
// encapsulate all the details into a common structure(SiddhiApp) regarless of the deployment type.
// Siddhi operator used this general SiddhiApp object to the further process.
func (p *Parser) Parse() (applications []deploymanager.Application, err error) {
	p.createRequest()
	err = p.deploy()
	appConfigs, err := p.invokeParser(p.Request)
	if err != nil {
		return
	}

	for i, appConfig := range appConfigs {
		var ports []corev1.ContainerPort
		var protocols []string
		var tls []bool
		siddhiApps := make(map[string]string)
		serviceEnabled := false
		siddhiApp := appConfig.SiddhiApp
		appName, err := getAppName(siddhiApp)
		deploymentName := p.SiddhiProcess.Name + "-" + strconv.Itoa(i)
		if err != nil {
			return applications, err
		}
		portList := []int{}
		for _, deploymentConf := range appConfig.DeploymentConfigs {
			if !deploymentConf.IsPulling && !ContainsInt(portList, deploymentConf.Port) {
				serviceEnabled = true
				port := corev1.ContainerPort{
					Name:          "p" + strconv.Itoa(deploymentConf.Port),
					ContainerPort: int32(deploymentConf.Port),
					Protocol:      corev1.Protocol(deploymentConf.ServiceProtocol),
				}
				ports = append(ports, port)
				portList = append(portList, deploymentConf.Port)
				protocols = append(protocols, deploymentConf.ServiceProtocol)
				tls = append(tls, deploymentConf.Secured)
			}
		}
		if len(ports) > 0 {
			serviceEnabled = true
		}
		siddhiApps[appName] = siddhiApp
		application := deploymanager.Application{
			Name:               deploymentName,
			ContainerPorts:     ports,
			Apps:               siddhiApps,
			ServiceEnabled:     serviceEnabled,
			PersistenceEnabled: appConfig.PersistenceEnabled,
			Replicas:           appConfig.Replicas,
		}
		applications = append(applications, application)
	}
	err = p.cleanParser()
	if err != nil {
		return
	}
	return applications, err
}

func (p *Parser) deploy() (err error) {
	generatedParserName := p.Name + ParserExtension
	containerPorts := []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          ParserName,
			ContainerPort: ParserPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
	application := deploymanager.Application{
		Name:           generatedParserName,
		ContainerPorts: containerPorts,
		ServiceEnabled: true,
		Replicas:       ParserReplicas,
	}

	deployManeger := deploymanager.DeployManager{
		Application:   application,
		KubeClient:    p.KubeClient,
		Image:         p.Image,
		SiddhiProcess: p.SiddhiProcess,
	}
	_, err = deployManeger.Deploy()
	if err != nil {
		return
	}
	_, err = p.KubeClient.CreateOrUpdateService(
		generatedParserName,
		p.SiddhiProcess.Namespace,
		containerPorts,
		deployManeger.Labels,
		p.SiddhiProcess,
	)
	if err != nil {
		return
	}

	url := ParserHTTP + generatedParserName + "." + p.SiddhiProcess.Namespace + ParserHealth
	p.Logger.Info("Waiting for parser", "deployment", p.Name)
	err = waitForParser(url)
	if err != nil {
		return
	}
	return
}

// invokeParser simply invoke the siddhi parser within the k8s cluster
func (p *Parser) invokeParser(
	request Request,
) (appConfig []AppConfig, err error) {
	url := ParserHTTP + p.Name + ParserExtension + "." + p.SiddhiProcess.Namespace + ParserContext
	b, err := json.Marshal(request)
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
	err = json.NewDecoder(resp.Body).Decode(&appConfig)
	if err != nil {
		return
	}
	return
}

// createRequest creates the request which send to the parser during the runtime.
func (p *Parser) createRequest() {
	request := Request{
		Apps:        p.Apps,
		PropertyMap: p.Env,
	}

	ms := siddhiv1alpha2.MessagingSystem{}
	if p.MessagingSystem.TypeDefined() {
		if p.MessagingSystem.EmptyConfig() {
			ms = siddhiv1alpha2.MessagingSystem{
				Type: NATSMessagingType,
				Config: siddhiv1alpha2.MessagingConfig{
					ClusterID: artifact.STANClusterName,
					BootstrapServers: []string{
						NATSDefaultURL,
					},
				},
			}
		} else {
			ms = p.MessagingSystem
		}
		request = Request{
			Apps:            p.Apps,
			PropertyMap:     p.Env,
			MessagingSystem: &ms,
		}
	}
	p.Request = request
}

// cleanParser simply remove the parser deployment and service created by the operator
func (p *Parser) cleanParser() (err error) {
	parserName := p.SiddhiProcess.Name + ParserExtension
	err = p.KubeClient.DeleteDeployment(parserName, p.SiddhiProcess.Namespace)
	if err != nil {
		return
	}
	err = p.KubeClient.DeleteService(parserName, p.SiddhiProcess.Namespace)
	if err != nil {
		return
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

// ContainsInt function to check given int is in the given slice or not
func ContainsInt(slice []int, value int) (contain bool) {
	contain = false
	for _, s := range slice {
		if s == value {
			contain = true
			return
		}
	}
	return
}
