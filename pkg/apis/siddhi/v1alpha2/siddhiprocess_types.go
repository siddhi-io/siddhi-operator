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

// MessagingConfig contains the configs of the messaging layer
type MessagingConfig struct {
	ClusterID        string   `json:"streamingClusterId"`
	BootstrapServers []string `json:"bootstrapServers"`
}

// MessagingSystem contains the details about the messaging layer
type MessagingSystem struct {
	Type   string          `json:"type"`
	Config MessagingConfig `json:"config"`
}

// Apps siddhi apps
type Apps struct {
	ConfigMap string `json:"configMap"`
	Script    string `json:"script"`
}

// PartialApp siddhi partial app
type PartialApp struct {
	DeploymentName string   `json:"deploymentName"`
	Apps           []string `json:"app"`
}

// SiddhiProcessSpec defines the desired state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessSpec struct {
	Apps            []Apps                           `json:"apps"`
	SiddhiConfig    string                           `json:"runner"`
	Container       corev1.Container                 `json:"container"`
	MessagingSystem MessagingSystem                  `json:"messagingSystem"`
	PVC             corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim"`
	ImagePullSecret string                           `json:"imagePullSecret"`
}

// SiddhiProcessStatus defines the observed state of SiddhiProcess
// +k8s:openapi-gen=true
type SiddhiProcessStatus struct {
	Status          string       `json:"status"`
	Ready           string       `json:"ready"`
	CurrentVersion  int64        `json:"currentVersion"`
	PreviousVersion int64        `json:"previousVersion"`
	EventType       string       `json:"eventType"`
	PartialApps     []PartialApp `json:"partialApps"`
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
