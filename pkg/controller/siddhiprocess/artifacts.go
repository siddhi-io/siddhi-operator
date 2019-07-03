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
	"reflect"
	"strconv"
	"strings"

	natsv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/nats/v1alpha2"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	streamingv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/streaming/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateConfigMap creates a k8s config map for given set of data.
// Inputs - SiddhiProcess object, name of the config map, & data object that used in the config map
// This function initialize the config map object, set the controller reference, and then creates the config map.
func (rsp *ReconcileSiddhiProcess) CreateConfigMap(sp *siddhiv1alpha2.SiddhiProcess, configMapName string, data map[string]string) error {
	configMap := &corev1.ConfigMap{}
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: sp.Namespace}, configMap)
	if err != nil && errors.IsNotFound(err) {
		configMap = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: sp.Namespace,
			},
			Data: data,
		}
		controllerutil.SetControllerReference(sp, configMap, rsp.scheme)
		err = rsp.client.Create(context.TODO(), configMap)
	}
	return err
}

// CreateIngress returns a Siddhi Ingress load balancer object
// Inputs - SiddhiProcess object, siddhi app struct to hold deployment configs, default config object, and the operator deployment object
func (rsp *ReconcileSiddhiProcess) CreateIngress(sp *siddhiv1alpha2.SiddhiProcess, siddhiApp SiddhiApp, configs Configs) (err error) {
	var ingressPaths []extensionsv1beta1.HTTPIngressPath
	for _, port := range siddhiApp.ContainerPorts {
		path := "/" + strings.ToLower(siddhiApp.Name) + "/" + strconv.Itoa(int(port.ContainerPort)) + "/"
		ingressPath := extensionsv1beta1.HTTPIngressPath{
			Path: path,
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: strings.ToLower(siddhiApp.Name),
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: port.ContainerPort,
				},
			},
		}
		ingressPaths = append(ingressPaths, ingressPath)
	}
	var ingressSpec extensionsv1beta1.IngressSpec
	if configs.IngressTLS != "" {
		ingressSpec = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				extensionsv1beta1.IngressTLS{
					Hosts:      []string{configs.HostName},
					SecretName: configs.IngressTLS,
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
	err = rsp.client.Create(context.TODO(), ingress)
	return
}

// UpdateIngress updates the ingress object and returns updated object
// Inputs - SiddhiProcess object, existing ingress object, siddhi app struct to hold deployment configs, and default configs
func (rsp *ReconcileSiddhiProcess) UpdateIngress(sp *siddhiv1alpha2.SiddhiProcess, currentIngress *extensionsv1beta1.Ingress, siddhiApp SiddhiApp, configs Configs) (err error) {
	var ingressPaths []extensionsv1beta1.HTTPIngressPath
	for _, port := range siddhiApp.ContainerPorts {
		path := "/" + strings.ToLower(siddhiApp.Name) + "/" + strconv.Itoa(int(port.ContainerPort)) + "/"
		ingressPath := extensionsv1beta1.HTTPIngressPath{
			Path: path,
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: strings.ToLower(siddhiApp.Name),
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: port.ContainerPort,
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
	if configs.IngressTLS != "" {
		ingressSpec = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				extensionsv1beta1.IngressTLS{
					Hosts:      []string{configs.HostName},
					SecretName: configs.IngressTLS,
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
	err = rsp.client.Update(context.TODO(), currentIngress)
	return
}

// CreateNATS function creates a NATS cluster and a NATS streaming cluster and waits some amout of time to complete it.
// More about NATS cluster - https://github.com/nats-io/nats-operator
// More about NATS streaming cluster - https://github.com/nats-io/nats-streaming-operator
func (rsp *ReconcileSiddhiProcess) CreateNATS(sp *siddhiv1alpha2.SiddhiProcess, configs Configs) error {
	natsCluster := &natsv1alpha2.NatsCluster{}
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.NATSClusterName, Namespace: sp.Namespace}, natsCluster)
	if err != nil && errors.IsNotFound(err) {
		natsCluster = &natsv1alpha2.NatsCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configs.NATSAPIVersion,
				Kind:       configs.NATSKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configs.NATSClusterName,
				Namespace: sp.Namespace,
			},
			Spec: natsv1alpha2.ClusterSpec{
				Size: configs.NATSSize,
			},
		}
		err = rsp.client.Create(context.TODO(), natsCluster)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	stanCluster := &streamingv1alpha1.NatsStreamingCluster{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.STANClusterName, Namespace: sp.Namespace}, stanCluster)
	if err != nil && errors.IsNotFound(err) {
		stanCluster = &streamingv1alpha1.NatsStreamingCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configs.STANAPIVersion,
				Kind:       configs.STANKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configs.STANClusterName,
				Namespace: sp.Namespace,
			},
			Spec: streamingv1alpha1.NatsStreamingClusterSpec{
				Size:        int32(configs.NATSSize),
				NatsService: configs.NATSClusterName,
			},
		}
		err = rsp.client.Create(context.TODO(), stanCluster)
		if err != nil {
			return err
		}
	}
	return err
}

// CreatePVC function creates a persistence volume claim for a K8s cluster
// Inputs - SiddhiProcess object, default configs, and name of the persistence volume claim
func (rsp *ReconcileSiddhiProcess) CreatePVC(sp *siddhiv1alpha2.SiddhiProcess, configs Configs, pvcName string) error {
	var accessModes []corev1.PersistentVolumeAccessMode
	pvc := &corev1.PersistentVolumeClaim{}
	p := sp.Spec.PV
	// todo default access mode
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: sp.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		for _, am := range p.AccessModes {
			if am == configs.ReadWriteOnce {
				accessModes = append(accessModes, corev1.ReadWriteOnce)
				continue
			}
			if am == configs.ReadOnlyMany {
				accessModes = append(accessModes, corev1.ReadOnlyMany)
				continue
				// throw error
			}
			if am == configs.ReadWriteMany {
				accessModes = append(accessModes, corev1.ReadWriteMany)
				continue
			}
		}
		pvc = &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: sp.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: accessModes,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(p.Resources.Requests.Storage),
					},
				},
				StorageClassName: &p.Class,
			},
		}
		controllerutil.SetControllerReference(sp, pvc, rsp.scheme)
		err = rsp.client.Create(context.TODO(), pvc)
	}
	return err
}

// CreateService returns a Service object for a deployment
// Inputs - SiddhiProcess object, SiddhiApp struct, envs of the operator deployment, default configs object
func (rsp *ReconcileSiddhiProcess) CreateService(sp *siddhiv1alpha2.SiddhiProcess, siddhiApp SiddhiApp, configs Configs) (err error) {
	labels := labelsForSiddhiProcess(strings.ToLower(siddhiApp.Name), configs)
	var servicePorts []corev1.ServicePort
	for _, containerPort := range siddhiApp.ContainerPorts {
		servicePort := corev1.ServicePort{
			Port: containerPort.ContainerPort,
			Name: containerPort.Name,
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
	err = rsp.client.Create(context.TODO(), service)
	if err != nil {
		return
	}
	return
}

// CreateDeployment creates a deployment for given set of configuration data.
func (rsp *ReconcileSiddhiProcess) CreateDeployment(
	sp *siddhiv1alpha2.SiddhiProcess,
	name string,
	namespace string,
	replicas int32,
	labels map[string]string,
	image string,
	containerName string,
	command []string,
	args []string,
	ports []corev1.ContainerPort,
	vms []corev1.VolumeMount,
	envs []corev1.EnvVar,
	sc corev1.SecurityContext,
	ipp corev1.PullPolicy,
	secrets []corev1.LocalObjectReference,
	volumes []corev1.Volume,
) (err error) {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            containerName,
							Command:         command,
							Args:            args,
							Ports:           ports,
							VolumeMounts:    vms,
							Env:             envs,
							SecurityContext: &sc,
							ImagePullPolicy: ipp,
						},
					},
					ImagePullSecrets: secrets,
					Volumes:          volumes,
				},
			},
		},
	}
	controllerutil.SetControllerReference(sp, deployment, rsp.scheme)
	err = rsp.client.Create(context.TODO(), deployment)
	if err != nil {
		return
	}
	return
}
