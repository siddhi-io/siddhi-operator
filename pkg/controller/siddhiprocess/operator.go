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
	appsv1 "k8s.io/api/apps/v1"
)

// populateOperatorEnvs returns a map of ENVs in the operator deployment
func (reconcileSiddhiProcess *ReconcileSiddhiProcess) populateOperatorEnvs(operatorDeployment *appsv1.Deployment) (envs map[string]string){
	envs = make(map[string]string)
	envStruct := operatorDeployment.Spec.Template.Spec.Containers[0].Env
	for _, env := range envStruct {
		envs[env.Name] = env.Value
	}
	
	return envs
}

