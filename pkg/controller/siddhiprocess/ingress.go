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
	"reflect"
	"strconv"
	"strings"

	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// IntOrString integer or string
type IntOrString struct {
	Type   Type   `protobuf:"varint,1,opt,name=type,casttype=Type"`
	IntVal int32  `protobuf:"varint,2,opt,name=intVal"`
	StrVal string `protobuf:"bytes,3,opt,name=strVal"`
}

// Type represents the stored type of IntOrString.
type Type int

// Int - Type
const (
	Int intstr.Type = iota
	String
)

// createIngress returns a Siddhi Ingress load balancer object
func (rsp *ReconcileSiddhiProcess) createIngress(sp *siddhiv1alpha1.SiddhiProcess, siddhiApp SiddhiApp, configs Configs, operator *appsv1.Deployment) *extensionsv1beta1.Ingress {
	var ingressPaths []extensionsv1beta1.HTTPIngressPath
	for _, port := range siddhiApp.Ports {
		path := "/" + strings.ToLower(siddhiApp.Name) + "/" + strconv.Itoa(port) + "/"
		ingressPath := extensionsv1beta1.HTTPIngressPath{
			Path: path,
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: strings.ToLower(siddhiApp.Name),
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: int32(port),
				},
			},
		}
		ingressPaths = append(ingressPaths, ingressPath)
	}
	var ingressSpec extensionsv1beta1.IngressSpec
	if sp.Spec.SiddhiIngressTLS.SecretName != "" {
		ingressSpec = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				extensionsv1beta1.IngressTLS{
					Hosts:      []string{configs.HostName},
					SecretName: sp.Spec.SiddhiIngressTLS.SecretName,
				},
			},
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: configs.HostName,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: ingressPaths,
						},
					},
				},
			},
		}
	} else {
		ingressSpec = extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: configs.HostName,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: ingressPaths,
						},
					},
				},
			},
		}
	}
	ingress := &extensionsv1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: extensionsv1beta1.SchemeGroupVersion.String(),
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configs.HostName,
			Namespace: sp.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                 "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target":  "/",
				"nginx.ingress.kubernetes.io/ssl-passthrough": "true",
			},
		},
		Spec: ingressSpec,
	}
	controllerutil.SetControllerReference(operator, ingress, rsp.scheme)
	return ingress
}

// updateIngress returns a Siddhi Ingress load balancer object
func (rsp *ReconcileSiddhiProcess) updateIngress(sp *siddhiv1alpha1.SiddhiProcess, currentIngress *extensionsv1beta1.Ingress, siddhiApp SiddhiApp, configs Configs) *extensionsv1beta1.Ingress {
	var ingressPaths []extensionsv1beta1.HTTPIngressPath
	for _, port := range siddhiApp.Ports {
		path := "/" + strings.ToLower(siddhiApp.Name) + "/" + strconv.Itoa(port) + "/"
		ingressPath := extensionsv1beta1.HTTPIngressPath{
			Path: path,
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: strings.ToLower(siddhiApp.Name),
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: int32(port),
				},
			},
		}
		ingressPaths = append(ingressPaths, ingressPath)
	}

	currentRules := currentIngress.Spec.Rules
	newRule := extensionsv1beta1.IngressRule{
		Host: configs.HostName,
		IngressRuleValue: extensionsv1beta1.IngressRuleValue{
			HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
				Paths: ingressPaths,
			},
		},
	}
	ruleExists := false
	for _, rule := range currentRules {
		if reflect.DeepEqual(rule, newRule) {
			ruleExists = true
		}
	}
	if !ruleExists {
		currentRules = append(currentRules, newRule)
	}
	var ingressSpec extensionsv1beta1.IngressSpec
	if sp.Spec.SiddhiIngressTLS.SecretName != "" {
		ingressSpec = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				extensionsv1beta1.IngressTLS{
					Hosts:      []string{configs.HostName},
					SecretName: sp.Spec.SiddhiIngressTLS.SecretName,
				},
			},
			Rules: currentRules,
		}
	} else {
		ingressSpec = extensionsv1beta1.IngressSpec{
			Rules: currentRules,
		}
	}
	currentIngress.Spec = ingressSpec
	return currentIngress
}
