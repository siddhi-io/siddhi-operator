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
	"strconv"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// serviceForSiddhi returns a Siddhi Service object
func (rsp *ReconcileSiddhiProcess) createService(sp *siddhiv1alpha1.SiddhiProcess, siddhiApp SiddhiApp, operatorEnvs map[string]string, configs Configs) *corev1.Service {
	labels := labelsForSiddhiProcess(strings.ToLower(siddhiApp.Name), operatorEnvs, configs)
	var servicePorts []corev1.ServicePort
	for _, port := range siddhiApp.Ports {
		servicePort := corev1.ServicePort{
			Port: int32(port),
			Name: strings.ToLower(siddhiApp.Name) + strconv.Itoa(port),
		}
		servicePorts = append(servicePorts, servicePort)
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      siddhiApp.Name,
			Namespace: sp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    servicePorts,
			Type:     "ClusterIP",
		},
	}
	controllerutil.SetControllerReference(sp, service, rsp.scheme)
	return service
}
