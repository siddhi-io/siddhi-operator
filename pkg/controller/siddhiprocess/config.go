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

// Default configs
const (
	SiddhiHome           string = "/home/siddhi_user/siddhi-runner-0.1.0/"
	SiddhiRunnerImage    string = "siddhiio/siddhi-runner-alpine"
	SiddhiRunnerImageTag string = "0.1.0"
	HostName             string = "siddhi"
	OperatorName         string = "siddhi-operator"
	OperatorVersion      string = "0.1.1"
	CRDName              string = "SiddhiProcess"
)

// Other constants
const (
	Push string = "PUSH"
	Pull string = "PULL"
)

// Configs contains siddhi default configs
type Configs struct {
	SiddhiHome           string
	SiddhiRunnerImage    string
	SiddhiRunnerImageTag string
	HostName             string
	OperatorName         string
	OperatorVersion      string
	CRDName              string
}

func configurations() Configs {
	configs := Configs{
		SiddhiHome:           SiddhiHome,
		SiddhiRunnerImage:    SiddhiRunnerImage,
		SiddhiRunnerImageTag: SiddhiRunnerImageTag,
		HostName:             HostName,
		OperatorName:         OperatorName,
		OperatorVersion:      OperatorVersion,
		CRDName:              CRDName,
	}
	return configs
}
