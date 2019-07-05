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
	"os"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Default configurations stored as constants. Further these constants used by the Configurations() function.
const (
	SiddhiHome         string = "/home/siddhi_user/"
	SiddhiImage        string = "siddhiio/siddhi-runner-alpine:5.1.0"
	SiddhiRunnerPath   string = "wso2/runner/"
	SiddhiCMExt        string = "-siddhi"
	SiddhiExt          string = ".siddhi"
	SiddhiFileRPath    string = "siddhi-files/"
	ContainerName      string = "siddhi-runner-runtime"
	DepConfigName      string = "deploymentconfig"
	DepConfMountPath   string = "tmp/configs/"
	DepConfParameter   string = "-Dconfig="
	AppConfParameter   string = "-Dconfig="
	DepCMExt           string = "-depyml"
	Shell              string = "sh"
	RunnerRPath        string = "init.sh"
	HostName           string = "siddhi"
	OperatorName       string = "siddhi-operator"
	OperatorVersion    string = "0.2.0"
	CRDName            string = "SiddhiProcess"
	ReadWriteOnce      string = "ReadWriteOnce"
	ReadOnlyMany       string = "ReadOnlyMany"
	ReadWriteMany      string = "ReadWriteMany"
	PVCExt             string = "-pvc"
	FilePersistentPath string = "siddhi-app-persistence"
	ParserDomain       string = "http://siddhi-parser."
	ParserContext      string = ".svc.cluster.local:9090/siddhi-parser/parse"
	PVCSize            string = "1Gi"
	NATSAPIVersion     string = "nats.io/v1alpha2"
	STANAPIVersion     string = "streaming.nats.io/v1alpha1"
	NATSKind           string = "NatsCluster"
	STANKind           string = "NatsStreamingCluster"
	NATSExt            string = "-nats"
	STANExt            string = "-stan"
	NATSClusterName    string = "siddhi-nats"
	STANClusterName    string = "siddhi-stan"
	NATSDefaultURL     string = "nats://siddhi-nats:4222"
	NATSTCPHost        string = "siddhi-nats:4222"
	NATSMSType         string = "nats"
	TCP                string = "tcp"
	IngressTLS         string = ""
	AutoCreateIngress  bool   = false
	NATSSize           int    = 1
	NATSTimeout        int    = 5
	DefaultRTime       int    = 1
	DeploymentSize     int32  = 1
)

// State persistence config is the different string constant used by the deployApp() function. This constant holds a YAML object
// which used to change the deployment.yaml file of the siddhi-runner image.
const (
	StatePersistenceConf string = `
state.persistence:
  enabled: true
  intervalInMin: 1
  revisionsToKeep: 2
  persistenceStore: io.siddhi.distribution.core.persistence.FileSystemPersistenceStore
  config:
    location: siddhi-app-persistence
`

	StatePersistenceConfTest string = `
state.persistence:
  enabled: true
  intervalInMin: 1
  revisionsToKeep: 2
  persistenceStore: io.siddhi.distribution.core.persistence.FileSystemPersistenceStore
  config:
    location: /home/siddhi
`
)

// These are all other relevant constants that used by the operator. But these constants are not configuration varibles.
// That is why this has been seperated.
const (
	Push           string = "PUSH"
	Pull           string = "PULL"
	Failover       string = "failover"
	Default        string = "default"
	Distributed    string = "distributed"
	ProcessApp     string = "process"
	PassthroughApp string = "passthrough"
	OperatorCMName string = "siddhi-operator"
)

// Int - Type
const (
	Int intstr.Type = iota
	String
)

// Type of status as list of integer constans
const (
	PENDING Status = iota
	READY
	RUNNING
	ERROR
	WARNING
	NORMAL
)

// Configs is the struct definition of the object which used to bundle the all default configurations
type Configs struct {
	SiddhiHome         string
	SiddhiImage        string
	SiddhiImageSecret  string
	SiddhiCMExt        string
	SiddhiExt          string
	SiddhiFileRPath    string
	SiddhiRunnerPath   string
	ContainerName      string
	DepConfigName      string
	DepConfMountPath   string
	DepConfParameter   string
	AppConfParameter   string
	DepCMExt           string
	Shell              string
	RunnerRPath        string
	HostName           string
	OperatorName       string
	OperatorVersion    string
	CRDName            string
	ReadWriteOnce      string
	ReadOnlyMany       string
	ReadWriteMany      string
	PVCExt             string
	FilePersistentPath string
	ParserDomain       string
	ParserContext      string
	PVCSize            string
	NATSAPIVersion     string
	STANAPIVersion     string
	NATSKind           string
	STANKind           string
	NATSExt            string
	STANExt            string
	NATSClusterName    string
	STANClusterName    string
	NATSDefaultURL     string
	NATSTCPHost        string
	NATSMSType         string
	TCP                string
	IngressTLS         string
	AutoCreateIngress  bool
	NATSSize           int
	NATSTimeout        int
	DefaultRTime       int
	DeploymentSize     int32
}

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

// IntOrString - integer or string
type IntOrString struct {
	Type   Type   `protobuf:"varint,1,opt,name=type,casttype=Type"`
	IntVal int32  `protobuf:"varint,2,opt,name=intVal"`
	StrVal string `protobuf:"bytes,3,opt,name=strVal"`
}

// Type represents the stored type of IntOrString.
type Type int

// SiddhiApp contains details about the siddhi app which need in the K8s deployment
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

// SiddhiParserRequest is request struct of the siddhi-parser
type SiddhiParserRequest struct {
	SiddhiApps      []string                        `json:"siddhiApps"`
	PropertyMap     map[string]string               `json:"propertyMap"`
	MessagingSystem *siddhiv1alpha2.MessagingSystem `json:"messagingSystem,omitempty"`
}

// SourceDeploymentConfig hold deployment configs of a particular siddhi app
type SourceDeploymentConfig struct {
	ServiceProtocol string `json:"serviceProtocol"`
	Secured         bool   `json:"secured"`
	Port            int    `json:"port"`
	IsPulling       bool   `json:"isPulling"`
}

// SiddhiAppConfig holds siddhi app and the relevant SourceList
type SiddhiAppConfig struct {
	SiddhiApp               string                   `json:"siddhiApp"`
	SourceDeploymentConfigs []SourceDeploymentConfig `json:"sourceDeploymentConfigs"`
	PersistenceEnabled      bool                     `json:"persistenceEnabled"`
	Replicas                int32                    `json:"replicas"`
}

// Status of a Siddhi process
type Status int

// Status array holds the string values of status
var status = []string{
	"Pending",
	"Ready",
	"Running",
	"Error",
	"Warning",
	"Normal",
}

// multiple siddhi apps to run tests
var app = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

var app1 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

var app2 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

// SiddhiProcess object used in tests
var testSP = &siddhiv1alpha2.SiddhiProcess{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "siddhi-app",
		Namespace: "default",
	},
	Spec: siddhiv1alpha2.SiddhiProcessSpec{
		Container: corev1.Container{
			Image: "siddhiio/siddhi-runner:5.1.0",
		},
		MessagingSystem: siddhiv1alpha2.MessagingSystem{
			Type: "nats",
		},
		PV: siddhiv1alpha2.PV{
			AccessModes: []string{
				"ReadWriteOnce",
			},
			VolumeMode: "Filesystem",
			Resources: siddhiv1alpha2.PVCResource{
				Requests: siddhiv1alpha2.PVCRequest{
					Storage: "1Gi",
				},
			},
			Class: "standard",
		},
	},
}

// SiddhiApp object used in tests
var testSiddhiApp = SiddhiApp{
	Name: "monitorapp",
	ContainerPorts: []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          "monitorapp8080",
			ContainerPort: 8080,
		},
	},
	Apps: map[string]string{
		"MonitorApp": app,
	},
	PersistenceEnabled: true,
}

// Configurations function returns the default config object. Here all the configs used as constants and budle together into a
// object and then returns that object. This object used to differenciate default configs from other variables.
func (rsp *ReconcileSiddhiProcess) Configurations(sp *siddhiv1alpha2.SiddhiProcess) Configs {
	configs := Configs{
		SiddhiHome:         SiddhiHome,
		SiddhiImage:        SiddhiImage,
		SiddhiCMExt:        SiddhiCMExt,
		SiddhiExt:          SiddhiExt,
		SiddhiFileRPath:    SiddhiFileRPath,
		SiddhiRunnerPath:   SiddhiRunnerPath,
		ContainerName:      ContainerName,
		DepConfigName:      DepConfigName,
		DepConfMountPath:   DepConfMountPath,
		DepConfParameter:   DepConfParameter,
		AppConfParameter:   AppConfParameter,
		DepCMExt:           DepCMExt,
		Shell:              Shell,
		RunnerRPath:        RunnerRPath,
		HostName:           HostName,
		OperatorName:       OperatorName,
		OperatorVersion:    OperatorVersion,
		CRDName:            CRDName,
		ReadWriteOnce:      ReadWriteOnce,
		ReadOnlyMany:       ReadOnlyMany,
		ReadWriteMany:      ReadWriteMany,
		PVCExt:             PVCExt,
		FilePersistentPath: FilePersistentPath,
		ParserDomain:       ParserDomain,
		ParserContext:      ParserContext,
		PVCSize:            PVCSize,
		NATSAPIVersion:     NATSAPIVersion,
		STANAPIVersion:     STANAPIVersion,
		NATSKind:           NATSKind,
		STANKind:           STANKind,
		NATSExt:            NATSExt,
		STANExt:            STANExt,
		NATSClusterName:    NATSClusterName,
		STANClusterName:    STANClusterName,
		NATSDefaultURL:     NATSDefaultURL,
		NATSTCPHost:        NATSTCPHost,
		NATSMSType:         NATSMSType,
		TCP:                TCP,
		IngressTLS:         IngressTLS,
		AutoCreateIngress:  AutoCreateIngress,
		NATSSize:           NATSSize,
		NATSTimeout:        NATSTimeout,
		DefaultRTime:       DefaultRTime,
		DeploymentSize:     DeploymentSize,
	}
	cmName := OperatorCMName
	env := os.Getenv("OPERATOR_CONFIGMAP")
	if env != "" {
		cmName = env
	}
	configMap := &corev1.ConfigMap{}
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: sp.Namespace}, configMap)
	if err == nil {
		if configMap.Data["siddhiRunnerHome"] != "" {
			configs.SiddhiHome = configMap.Data["siddhiRunnerHome"]
		}

		if configMap.Data["siddhiRunnerImage"] != "" {
			configs.SiddhiImage = configMap.Data["siddhiRunnerImage"]
		}

		if configMap.Data["siddhiRunnerImageSecret"] != "" {
			configs.SiddhiImageSecret = configMap.Data["siddhiRunnerImageSecret"]
		}

		if configMap.Data["autoIngressCreation"] != "" {
			if configMap.Data["autoIngressCreation"] == "true" {
				configs.AutoCreateIngress = true
			}
		}

		if configMap.Data["ingressTLS"] != "" {
			configs.IngressTLS = configMap.Data["ingressTLS"]
		}

	}
	return configs
}
