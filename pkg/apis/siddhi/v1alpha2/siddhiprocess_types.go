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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TLS contains the TLS configuration of ingress
type TLS struct {
	SecretName string `json:"ingressSecret"`
}

// PVCRequest hold resource details of the PVC
type PVCRequest struct {
	Storage string `json:"storage"`
}

// PVCResource contains PVC resource details
type PVCResource struct {
	Requests PVCRequest `json:"requests"`
}

// PV contains the configurations of the persistence volume
type PV struct {
	AccessModes []string    `json:"accessModes"`
	VolumeMode  string      `json:"volumeMode"`
	Class       string      `json:"storageClassName"`
	Resources   PVCResource `json:"resources"`
}

// MessagingConfig contains the configs of the messaging layer
type MessagingConfig struct {
	ClusterID        string   `json:"clusterId"`
	BootstrapServers []string `json:"bootstrapServers"`
}

// MessagingSystem contains the details about the messaging layer
type MessagingSystem struct {
	Type   string          `json:"type"`
	Config MessagingConfig `json:"config"`
}

// DeploymentConfig contains the config details of the SiddhiProcess deployment
type DeploymentConfig struct {
	Mode            string          `json:"mode"`
	MessagingSystem MessagingSystem `json:"messagingSystem"`
	PV              PV              `json:"persistenceVolume"`
}

// Apps siddhi apps
type Apps struct {
	ConfigMap string `json:"configMap"`
	Script    string `json:"script"`
}

// SiddhiProcessSpec defines the desired state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessSpec struct {
	Apps            []Apps           `json:"apps"`
	SiddhiConfig    string           `json:"runner"`
	Container       corev1.Container `json:"container"`
	MessagingSystem MessagingSystem  `json:"messagingSystem"`
	PV              PV               `json:"persistentVolume"`
	ImagePullSecret string           `json:"imagePullSecret"`
}

// SiddhiProcessStatus defines the observed state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessStatus struct {
	Status string   `json:"status"`
	Ready  string   `json:"ready"`
	CurrentVersion int64 `json:"currentVersion"`
	PreviousVersion int64 `json:"previousVersion"`
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
