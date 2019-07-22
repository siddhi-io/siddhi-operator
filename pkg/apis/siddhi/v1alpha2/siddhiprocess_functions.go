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
	"sort"
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

// Equals function of PV check the equality of two PV structs
func (p *PV) Equals(q *PV) bool {
	vmEq := p.VolumeMode == q.VolumeMode
	classEq := p.Class == q.Class
	resourceEq := p.Resources == q.Resources
	if len(p.AccessModes) != len(q.AccessModes) {
		return false
	}
	sort.Strings(p.AccessModes)
	sort.Strings(q.AccessModes)
	for i := range p.AccessModes {
		if p.AccessModes[i] != q.AccessModes[i] {
			return false
		}
	}
	return (vmEq && classEq && resourceEq)
}

// Equals function of MessagingSystem check the equality of two MessagingSystem structs
func (p *MessagingSystem) Equals(q *MessagingSystem) bool {
	typeEq := p.Type == q.Type
	cidEq := p.Config.ClusterID == q.Config.ClusterID
	if len(p.Config.BootstrapServers) != len(q.Config.BootstrapServers) {
		return false
	}
	sort.Strings(p.Config.BootstrapServers)
	sort.Strings(q.Config.BootstrapServers)
	for i := range p.Config.BootstrapServers {
		if p.Config.BootstrapServers[i] != q.Config.BootstrapServers[i] {
			return false
		}
	}
	return (typeEq && cidEq)
}

// Equals 
func (p *SiddhiProcessSpec) Equals(q *SiddhiProcessSpec) bool {
	if !EqualApps(p.Apps, q.Apps) {
		return false
	}
	if p.SiddhiConfig != q.SiddhiConfig {
		return false
	}
	if !EqualContainers(&p.Container, &q.Container){
		return false
	}
	if !p.MessagingSystem.Equals(&q.MessagingSystem) {
		return false
	}
	if !p.PV.Equals(&q.PV) {
		return false
	}
	if p.ImagePullSecret != q.ImagePullSecret {
		return false
	}
	return true
}

// EqualApps 
func EqualApps(p []Apps, q []Apps) bool {
	if len(p) != len(q) {
		return false
	}
	for _, pApp := range p {
		contained := false
		for _, qApp := range q {
			if reflect.DeepEqual(pApp, qApp) {
				contained = true
				break
			}
		}
		if !contained {
			return false
		}
	}
	return true
}

// EqualContainers 
func EqualContainers(p *corev1.Container, q *corev1.Container) bool {
	if p.Image != q.Image {
		return false
	}
	for _, pEnv := range p.Env {
		contained := false
		for _, qEnv := range q.Env {
			if reflect.DeepEqual(pEnv, qEnv) {
				contained = true
				break
			}
		}
		if !contained {
			return false
		}
	}
	return true
}

// EmptyConfig function of MessagingSystem check the equality of two MessagingSystem structs
func (p *MessagingSystem) EmptyConfig() bool {
	if p.Config.ClusterID != "" {
		return false
	}
	if len(p.Config.BootstrapServers) > 0 {
		return false
	}
	return true
}

// TypeDefined function of MessagingSystem check the equality of two MessagingSystem structs
func (p *MessagingSystem) TypeDefined() bool {
	if p.Type != "" {
		return true
	}
	return false
}
