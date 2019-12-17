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

package artifacts

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Constants for the Kubernetes artifacts
const (
	IngressName     string = "siddhi"
	NATSClusterName string = "siddhi-nats"
	STANClusterName string = "siddhi-stan"
	NATSAPIVersion  string = "nats.io/v1alpha2"
	STANAPIVersion  string = "streaming.nats.io/v1alpha1"
	NATSKind        string = "NatsCluster"
	STANKind        string = "NatsStreamingCluster"
	NATSClusterSize int    = 1
	STANClusterSize int32  = 1
	GigaByte        int64  = 1024 * 1024 * 1024
)

// Constants for K8s deployment
const (
	HealthPath                 string = "/health"
	HealthPortName             string = "hport"
	HealthPort                 int32  = 9090
	ReadyPrPeriodSeconds       int32  = 10
	ReadyPrInitialDelaySeconds int32  = 10
	LivePrPeriodSeconds        int32  = 70
	LivePrInitialDelaySeconds  int32  = 20
)

// Int - Type
const (
	Int intstr.Type = iota
	String
)
