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

package siddhicontroller

// Controller configs
const (
	OperatorCMName    string = "siddhi-operator-config"
	SiddhiHome        string = "/home/siddhi_user/siddhi-runner/"
	SiddhiImage       string = "siddhiio/siddhi-runner-alpine:5.1.2"
	SiddhiProfile     string = "runner"
	AutoCreateIngress bool   = true
)

// Type of status as list of integer constans
const (
	PENDING Status = iota
	READY
	RUNNING
	ERROR
	WARNING
	NORMAL
	NOTREADY
)
