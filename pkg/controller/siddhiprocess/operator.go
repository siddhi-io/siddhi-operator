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
	"errors"
	"fmt"
	"strconv"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
)

// populateUserEnvs returns a map for the ENVs in CRD
func (rsp *ReconcileSiddhiProcess) populateUserEnvs(sp *siddhiv1alpha1.SiddhiProcess) (envs map[string]string) {
	envs = make(map[string]string)
	for _, env := range sp.Spec.Container.Env {
		envs[env.Name] = env.Value
	}
	return envs
}

// updateStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Pending, Warning, Error, Running
func (rsp *ReconcileSiddhiProcess) updateErrorStatus(sp *siddhiv1alpha1.SiddhiProcess, eventRecorder record.EventRecorder, status Status, reason string, er error) (s *siddhiv1alpha1.SiddhiProcess) {
	s = &siddhiv1alpha1.SiddhiProcess{}
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}, s)
	if err != nil {
		return sp
	}
	st := getStatus(status)
	s.Status.Status = st

	if status == ERROR || status == WARNING {
		eventRecorder.Event(sp, getStatus(WARNING), reason, er.Error())
		if status == ERROR {
			reqLogger.Error(er, er.Error())
		} else {
			reqLogger.Info(er.Error())
		}
	}
	// err = rsp.client.Status().Update(context.TODO(), s)
	err = rsp.client.Status().Update(context.TODO(), s)
	if err != nil {
		fmt.Println("###################")
		fmt.Println(err)
		return sp
	}
	return s
}

// updateStatus update the status of the CR object and send events to the SiddhiProcess object using EventRecorder object
// These status can be Pending, Warning, Error, Running
func (rsp *ReconcileSiddhiProcess) updateRunningStatus(sp *siddhiv1alpha1.SiddhiProcess, eventRecorder record.EventRecorder, status Status, reason string, message string) (s *siddhiv1alpha1.SiddhiProcess) {
	s = &siddhiv1alpha1.SiddhiProcess{}
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}, s)
	if err != nil {
		return sp
	}
	st := getStatus(status)
	s.Status.Status = st
	if status == RUNNING {
		eventRecorder.Event(sp, getStatus(NORMAL), reason, message)
		reqLogger.Info(message)
	}
	err = rsp.client.Status().Update(context.TODO(), s)
	if err != nil {
		return sp
	}
	return s
}

// updateType update the deployment type of the CR object
// These types are default, failover, and distributed
func (rsp *ReconcileSiddhiProcess) updateType(sp *siddhiv1alpha1.SiddhiProcess, deptType string) (s *siddhiv1alpha1.SiddhiProcess) {
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
// Ready attribute contains the number of deployments are complete and running out of requested deployments
func (rsp *ReconcileSiddhiProcess) updateReady(sp *siddhiv1alpha1.SiddhiProcess, available int, need int) (s *siddhiv1alpha1.SiddhiProcess) {
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

func (rsp *ReconcileSiddhiProcess) createArtifacts(sp *siddhiv1alpha1.SiddhiProcess, siddhiApps []SiddhiApp, configs Configs) *siddhiv1alpha1.SiddhiProcess {
	needDep := 0
	availableDep := 0
	reqLogger := log.WithValues("Request.Namespace", sp.Namespace, "Request.Name", sp.Name)
	for _, siddhiApp := range siddhiApps {
		needDep++
		deployment := &appsv1.Deployment{}
		err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err != nil && apierrors.IsNotFound(err) {
			err = rsp.deployApp(sp, siddhiApp, ER, configs)
			if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "AppDeploymentError", err)
				continue
			}
			availableDep++
			sp = rsp.updateRunningStatus(sp, ER, RUNNING, "DeploymentCreated", (siddhiApp.Name + " deployment created successfully"))
		} else if err != nil {
			sp = rsp.updateErrorStatus(sp, ER, ERROR, "DeploymentNotFound", err)
			continue
		} else {
			availableDep++
		}

		if siddhiApp.ServiceEnabled {
			service := &corev1.Service{}
			err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: siddhiApp.Name, Namespace: sp.Namespace}, service)
			if err != nil && apierrors.IsNotFound(err) {
				err := rsp.createService(sp, siddhiApp, configs)
				if err != nil {
					sp = rsp.updateErrorStatus(sp, ER, WARNING, "ServiceCreationError", err)
					continue
				}
				sp = rsp.updateRunningStatus(sp, ER, RUNNING, "ServiceCreated", (siddhiApp.Name + " service created successfully"))
			} else if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "ServiceNotFound", err)
				continue
			}

			if configs.AutoCreateIngress {
				ingress := &extensionsv1beta1.Ingress{}
				err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.HostName, Namespace: sp.Namespace}, ingress)
				if err != nil && apierrors.IsNotFound(err) {
					err := rsp.createIngress(sp, siddhiApp, configs)
					if err != nil {
						sp = rsp.updateErrorStatus(sp, ER, ERROR, "IngressCreationError", err)
						continue
					}
					reqLogger.Info("New ingress created successfully", "Ingress.Name", configs.HostName)
				} else if err != nil {
					err := rsp.updateIngress(sp, ingress, siddhiApp, configs)
					if err != nil {
						sp = rsp.updateErrorStatus(sp, ER, ERROR, "IngressUpdationError", err)
						continue
					}
					reqLogger.Info("Ingress updated successfully", "Ingress.Name", configs.HostName)
				}
			}
		}
	}
	sp = rsp.updateReady(sp, availableDep, needDep)
	return sp
}

func (rsp *ReconcileSiddhiProcess) checkDeployments(sp *siddhiv1alpha1.SiddhiProcess, siddhiApps []SiddhiApp) *siddhiv1alpha1.SiddhiProcess {
	for _, siddhiApp := range siddhiApps {
		deployment := &appsv1.Deployment{}
		err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(siddhiApp.Name), Namespace: sp.Namespace}, deployment)
		if err == nil && *deployment.Spec.Replicas != siddhiApp.Replicas {
			deployment.Spec.Replicas = &siddhiApp.Replicas
			err = rsp.client.Update(context.TODO(), deployment)
			if err != nil {
				sp = rsp.updateErrorStatus(sp, ER, ERROR, "DeploymentUpdationError", err)
				continue
			}
		}
	}
	return sp
}

func (rsp *ReconcileSiddhiProcess) populateSiddhiApps(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) (siddhiApps []SiddhiApp, err error) {
	if _, ok := SPContainer[sp.Name]; ok {
		siddhiApps = SPContainer[sp.Name]
	} else {
		siddhiApps, err = rsp.parseApp(sp, configs)
		if err != nil {
			return
		}
		SPContainer[sp.Name] = siddhiApps
	}
	return
}

func (rsp *ReconcileSiddhiProcess) createMessagingSystem(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) (err error) {
	if sp.Spec.DeploymentConfig.Mode == Failover {
		sp = rsp.updateType(sp, Failover)
		ms := siddhiv1alpha1.MessagingSystem{}
		if sp.Spec.DeploymentConfig.MessagingSystem.Equals(&ms) {
			err = rsp.createNATS(sp, configs)
			if err != nil {
				return
			}
		}
	} else {
		sp = rsp.updateType(sp, Default)
	}
	return
}

func (rsp *ReconcileSiddhiProcess) getSiddhiApps(sp *siddhiv1alpha1.SiddhiProcess) (siddhiApps []string, err error) {
	fromString := sp.Spec.Apps.FromString
	fromConfigMaps := sp.Spec.Apps.FromConfigMaps
	if (fromString == "") && (len(fromConfigMaps) > 0) {
		for _, siddhiCMName := range sp.Spec.Apps.FromConfigMaps {
			configMap := &corev1.ConfigMap{}
			rsp.client.Get(context.TODO(), types.NamespacedName{Name: siddhiCMName, Namespace: sp.Namespace}, configMap)
			for _, siddhiFileContent := range configMap.Data {
				siddhiApps = append(siddhiApps, siddhiFileContent)
			}
		}
	} else if (fromString != "") && (len(fromConfigMaps) <= 0) {
		siddhiApps = append(siddhiApps, fromString)
	} else if (fromString != "") && (len(sp.Spec.Apps.FromConfigMaps) > 0) {
		err = errors.New("Custom resource " + sp.Name + " should only contain either query or app entry")
	} else {
		err = errors.New("Custom resource " + sp.Name + " must have either query or app entry to deploy siddhi apps")
	}
	return
}
