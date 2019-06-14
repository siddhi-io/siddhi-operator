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
	"context"
	"strconv"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
)

// populateOperatorEnvs returns a map of ENVs in the operator deployment
func (rsp *ReconcileSiddhiProcess) populateOperatorEnvs(operatorDeployment *appsv1.Deployment) (envs map[string]string) {
	envs = make(map[string]string)
	envStruct := operatorDeployment.Spec.Template.Spec.Containers[0].Env
	for _, env := range envStruct {
		envs[env.Name] = env.Value
	}

	return envs
}

// populateUserEnvs returns a map for the ENVs in CRD
func (rsp *ReconcileSiddhiProcess) populateUserEnvs(sp *siddhiv1alpha1.SiddhiProcess) (envs map[string]string) {
	envs = make(map[string]string)
	envStruct := sp.Spec.EnviromentVariables
	for _, env := range envStruct {
		envs[env.Name] = env.Value
	}
	return envs
}

// updateStatus update the status of the CR object
func (rsp *ReconcileSiddhiProcess) updateStatus(n Status, reason string, message string, eventRecorder record.EventRecorder, er error, sp *siddhiv1alpha1.SiddhiProcess) (s *siddhiv1alpha1.SiddhiProcess) {
	s = &siddhiv1alpha1.SiddhiProcess{}
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}, s)
	if err != nil {
		return sp
	}
	st := getStatus(n)
	s.Status.Status = st

	if n == ERROR || n == WARNING {
		eventRecorder.Event(sp, getStatus(WARNING), reason, message)
		if n == ERROR {
			reqLogger.Error(er, message)
		} else {
			reqLogger.Info(message)
		}
	}
	err = rsp.client.Status().Update(context.TODO(), s)
	if err != nil {
		return sp
	}
	return s
}

// updateType update the type of the CR object
func (rsp *ReconcileSiddhiProcess) updateType(deptType string, sp *siddhiv1alpha1.SiddhiProcess) (s *siddhiv1alpha1.SiddhiProcess) {
	s = &siddhiv1alpha1.SiddhiProcess{}
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}, s)
	if err != nil {
		return sp
	}
	s.Status.Type = deptType
	err = rsp.client.Status().Update(context.TODO(), s)
	if err != nil {
		return sp
	}
	return s
}

// updateReady update ready attribute of the CR object
func (rsp *ReconcileSiddhiProcess) updateReady(available int, need int, sp *siddhiv1alpha1.SiddhiProcess) (s *siddhiv1alpha1.SiddhiProcess) {
	s = &siddhiv1alpha1.SiddhiProcess{}
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}, s)
	if err != nil {
		return sp
	}
	s.Status.Ready = strconv.Itoa(available) + "/" + strconv.Itoa(need)
	err = rsp.client.Status().Update(context.TODO(), s)
	if err != nil {
		return sp
	}
	return s
}
