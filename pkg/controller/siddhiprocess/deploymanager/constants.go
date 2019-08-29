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

package deploymanager

// Constants for the DeployManager
const (
	OperatorName     string = "siddhi-operator"
	OperatorVersion  string = "0.2.0-alpha"
	CRDName          string = "SiddhiProcess"
	PVCExtension     string = "-pvc"
	DepCMExtension   string = "-depyml"
	SiddhiExtension  string = ".siddhi"
	DepConfMountPath string = "tmp/configs/"
	SiddhiFilesDir   string = "siddhi-files/"
	DepConfParameter string = "-Dconfig="
	AppConfParameter string = "-Dapps="
	ParserParameter  string = "-Dsiddhi-parser "
	ContainerName    string = "siddhi-runner-runtime"
	Shell            string = "sh"
	SiddhiBin        string = "bin"
	MaxUnavailable   int32  = 0
	MaxSurge         int32  = 2
)

// Default directories in the docker image
const (
	WSO2Dir           string = "wso2"
	FilePersistentDir string = "siddhi-app-persistence"
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
)
