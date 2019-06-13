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
	"errors"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Status of a Siddhi process
type Status int

// Type of status
const (
	PENDING Status = iota
	READY
	RUNNING
	ERROR
	WARNING
)

// labelsForSiddhiProcess returns the labels for selecting the resources
// belonging to the given sp CR name.
func labelsForSiddhiProcess(appName string, operatorEnvs map[string]string, configs Configs) map[string]string {
	operatorName := configs.OperatorName
	operatorVersion := configs.OperatorVersion
	if operatorEnvs["OPERATOR_NAME"] != "" {
		operatorName = operatorEnvs["OPERATOR_NAME"]
	}
	if operatorEnvs["OPERATOR_VERSION"] != "" {
		operatorVersion = operatorEnvs["OPERATOR_VERSION"]
	}
	return map[string]string{
		"siddhi.io/name":     configs.CRDName,
		"siddhi.io/instance": appName,
		"siddhi.io/version":  operatorVersion,
		"siddhi.io/part-of":  operatorName,
	}
}

// podNames returns the pod names of the array of pods passed in
func podNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// Status array
var status = []string{
	"Pending",
	"Ready",
	"Running",
	"Error",
	"Warning",
}

// getStatus return relevant status to a given int
func getStatus(n Status) string {
	return status[n]
}

// GetAppName return the app name for given siddhiAPP
func GetAppName(app string) (appName string, err error) {
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
