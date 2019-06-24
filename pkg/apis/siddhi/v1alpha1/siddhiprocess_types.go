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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnviromentVariable to store env name and value
type EnviromentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TLS contains the TLS configuration of ingress
type TLS struct {
	SecretName string `json:"ingressSecret"`
}

// Pod contains the POD details
type Pod struct {
	Image           string `json:"image"`
	ImageTag        string `json:"imageTag"`
	ImagePullSecret string `json:"imagePullSecret"`
}

// PVCRequest hold resource details of the PVC
type PVCRequest struct {
	Storage string `json:"storage"`
}

// PVCResource contains PVC resource details
type PVCResource struct {
	Requests PVCRequest `json:"requests"`
}

// PersistenceVolume contains the configurations of the persistence volume
type PersistenceVolume struct {
	AccessModes []string    `json:"access.modes"`
	VolumeMode  string      `json:"volume.mode"`
	Class       string      `json:"storageClassName"`
	Resources   PVCResource `json:"resources"`
}

// MessagingSystemConfig contains the configs of the messaging layer
type MessagingSystemConfig struct {
	ClusterID        string   `json:"cluster.id"`
	BootstrapServers []string `json:"bootstrap.servers"`
}

// MessagingSystem contains the details about the messaging layer
type MessagingSystem struct {
	Type   string                `json:"type"`
	Config MessagingSystemConfig `json:"config"`
}

// DeploymentConfigs contains the config details of the SiddhiProcess deployment
type DeploymentConfigs struct {
	Mode              string            `json:"mode"`
	MessagingSystem   MessagingSystem   `json:"messaging.system"`
	PersistenceVolume PersistenceVolume `json:"persistence.volume"`
}

// SiddhiProcessSpec defines the desired state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessSpec struct {
	Apps                []string             `json:"apps"`
	Query               string               `json:"query"`
	SiddhiConfig        string               `json:"siddhi.runner.configs"`
	EnviromentVariables []EnviromentVariable `json:"env"`
	SiddhiIngressTLS    TLS                  `json:"tls"`
	SiddhiPod           Pod                  `json:"pod"`
	DeploymentConfigs   DeploymentConfigs    `json:"deployment.configs"`
}

// SiddhiProcessStatus defines the observed state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessStatus struct {
	Nodes  []string `json:"nodes"`
	Status string   `json:"status"`
	Type   string   `json:"type"`
	Ready  string   `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiddhiProcess is the Schema for the siddhiprocesses API
// +k8s:openapi-gen=true
type SiddhiProcess struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SiddhiProcessSpec   `json:"spec,omitempty"`
	Status SiddhiProcessStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiddhiProcessList contains a list of SiddhiProcess
type SiddhiProcessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SiddhiProcess `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SiddhiProcess{}, &SiddhiProcessList{})
}
