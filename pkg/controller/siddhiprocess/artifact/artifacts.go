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

package artifacts

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"

	natsv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/nats/v1alpha2"
	streamingv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/streaming/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// KubeClient performs CRUD operations in the K8s cluster
type KubeClient struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// CreateOrUpdateCM creates a k8s config map for the given set of data.
// This function initializes the config map object, set the controller reference, and then creates the config map.
func (k *KubeClient) CreateOrUpdateCM(
	name string,
	namespace string,
	data map[string]string,
	owner metav1.Object,
) error {
	configMap := &corev1.ConfigMap{}
	err := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		configMap,
	)
	configMap = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	controllerutil.SetControllerReference(owner, configMap, k.Scheme)
	_, err = controllerutil.CreateOrUpdate(context.TODO(), k.Client, configMap, ConfigMapMutateFunc(data))
	return err
}

// CreateOrUpdateIngress creates an NGINX Ingress load balancer object called siddhi
func (k *KubeClient) CreateOrUpdateIngress(
	namespace string,
	serviceName string,
	tls string,
	containerPorts []corev1.ContainerPort,
) (operationResult controllerutil.OperationResult, err error) {

	var ingressPaths []extensionsv1beta1.HTTPIngressPath
	for _, port := range containerPorts {
		path := "/" + strings.ToLower(serviceName) +
			"/" + strconv.Itoa(int(port.ContainerPort)) + "(/|$)(.*)"
		ingressPath := extensionsv1beta1.HTTPIngressPath{
			Path: path,
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: strings.ToLower(serviceName),
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: port.ContainerPort,
				},
			},
		}
		ingressPaths = append(ingressPaths, ingressPath)
	}
	var ingressSpec extensionsv1beta1.IngressSpec
	if tls != "" {
		ingressSpec = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				extensionsv1beta1.IngressTLS{
					Hosts:      []string{IngressName},
					SecretName: tls,
				},
			},
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: IngressName,
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
					Host: IngressName,
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
			Name:      IngressName,
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
			},
		},
		Spec: ingressSpec,
	}
	operationResult, err = controllerutil.CreateOrUpdate(
		context.TODO(),
		k.Client,
		ingress,
		IngressMutateFunc(
			namespace,
			serviceName,
			tls,
			containerPorts,
		),
	)
	return
}

// CreateNATS function creates a NATS cluster and a NATS streaming cluster
// More about NATS cluster - https://github.com/nats-io/nats-operator
// More about NATS streaming cluster - https://github.com/nats-io/nats-streaming-operator
func (k *KubeClient) CreateNATS(namespace string) error {
	natsCluster := &natsv1alpha2.NatsCluster{}
	err := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: NATSClusterName, Namespace: namespace},
		natsCluster,
	)
	if err != nil && apierrors.IsNotFound(err) {
		natsCluster = &natsv1alpha2.NatsCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: NATSAPIVersion,
				Kind:       NATSKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      NATSClusterName,
				Namespace: namespace,
			},
			Spec: natsv1alpha2.ClusterSpec{
				Size: NATSClusterSize,
			},
		}
		err = k.Client.Create(context.TODO(), natsCluster)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	stanCluster := &streamingv1alpha1.NatsStreamingCluster{}
	err = k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: STANClusterName, Namespace: namespace},
		stanCluster,
	)
	if err != nil && apierrors.IsNotFound(err) {
		stanCluster = &streamingv1alpha1.NatsStreamingCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: STANAPIVersion,
				Kind:       STANKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      STANClusterName,
				Namespace: namespace,
			},
			Spec: streamingv1alpha1.NatsStreamingClusterSpec{
				Size:        STANClusterSize,
				NatsService: NATSClusterName,
			},
		}
		err = k.Client.Create(context.TODO(), stanCluster)
		if err != nil {
			return err
		}
	}
	return err
}

// CreateOrUpdatePVC function creates a persistence volume claim in a K8s cluster
func (k *KubeClient) CreateOrUpdatePVC(
	name string,
	namespace string,
	accessModes []corev1.PersistentVolumeAccessMode,
	storage string,
	storageClassName string,
	owner metav1.Object,
) error {
	pvc := &corev1.PersistentVolumeClaim{}
	err := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		pvc,
	)
	if err != nil && apierrors.IsNotFound(err) {
		accessGranted := false
		for _, accessMode := range accessModes {
			if accessMode == corev1.ReadWriteOnce {
				accessGranted = true
				break
			}
			if accessMode == corev1.ReadWriteMany {
				accessGranted = true
				break
			}
		}
		if !accessGranted {
			return errors.New("Restricted access mode in " + name)
		}
		pvc = &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: accessModes,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(storage),
					},
				},
				StorageClassName: &storageClassName,
			},
		}
		controllerutil.SetControllerReference(owner, pvc, k.Scheme)
		_, err = controllerutil.CreateOrUpdate(
			context.TODO(),
			k.Client,
			pvc,
			PVCMutateFunc(accessModes, storage, storageClassName),
		)
	}
	return err
}

// CreateOrUpdateService returns a Service object for deployment
func (k *KubeClient) CreateOrUpdateService(
	name string,
	namespace string,
	containerPorts []corev1.ContainerPort,
	selectors map[string]string,
	owner metav1.Object,
) (operationResult controllerutil.OperationResult, err error) {
	var servicePorts []corev1.ServicePort
	for _, containerPort := range containerPorts {
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
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectors,
			Ports:    servicePorts,
			Type:     "ClusterIP",
		},
	}
	controllerutil.SetControllerReference(owner, service, k.Scheme)
	operationResult, err = controllerutil.CreateOrUpdate(
		context.TODO(),
		k.Client,
		service,
		ServiceMutateFunc(selectors, servicePorts),
	)
	return
}

// CreateOrUpdateDeployment creates a deployment for a given set of configuration data
func (k *KubeClient) CreateOrUpdateDeployment(
	name string,
	namespace string,
	replicas int32,
	labels map[string]string,
	image string,
	containerName string,
	command []string,
	args []string,
	ports []corev1.ContainerPort,
	volumeMounts []corev1.VolumeMount,
	envs []corev1.EnvVar,
	sc corev1.SecurityContext,
	ipp corev1.PullPolicy,
	secrets []corev1.LocalObjectReference,
	volumes []corev1.Volume,
	strategy appsv1.DeploymentStrategy,
	owner metav1.Object,
) (operationResult controllerutil.OperationResult, err error) {
	httpGetAction := corev1.HTTPGetAction{
		Path: HealthPath,
		Port: intstr.IntOrString{
			Type:   Int,
			IntVal: HealthPort,
		},
	}
	readyProbe := corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &httpGetAction,
		},
		PeriodSeconds:       ReadyPrPeriodSeconds,
		InitialDelaySeconds: ReadyPrInitialDelaySeconds,
	}
	liveProbe := corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &httpGetAction,
		},
		PeriodSeconds:       LivePrPeriodSeconds,
		InitialDelaySeconds: LivePrInitialDelaySeconds,
	}
	defaultPort := corev1.ContainerPort{
		Name:          HealthPortName,
		ContainerPort: HealthPort,
		Protocol:      corev1.ProtocolTCP,
	}
	ports = append(ports, defaultPort)
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
						corev1.Container{
							Image:           image,
							Name:            containerName,
							Command:         command,
							Args:            args,
							Ports:           ports,
							VolumeMounts:    volumeMounts,
							Env:             envs,
							SecurityContext: &sc,
							ImagePullPolicy: ipp,
							ReadinessProbe:  &readyProbe,
							LivenessProbe:   &liveProbe,
						},
					},
					ImagePullSecrets: secrets,
					Volumes:          volumes,
				},
			},
			Strategy: strategy,
		},
	}
	controllerutil.SetControllerReference(owner, deployment, k.Scheme)
	operationResult, err = controllerutil.CreateOrUpdate(
		context.TODO(),
		k.Client,
		deployment,
		DeploymentMutateFunc(
			replicas,
			labels,
			image,
			containerName,
			command,
			args,
			ports,
			volumeMounts,
			envs,
			sc,
			ipp,
			secrets,
			volumes,
		),
	)
	return
}

// DeleteService deletes the service specified by the user
func (k *KubeClient) DeleteService(
	name string,
	namespace string,
) (err error) {
	service := &corev1.Service{}
	er := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		service,
	)
	if er == nil {
		err = k.Client.Delete(context.TODO(), service)
		if err != nil {
			return
		}
	}
	return
}

// DeleteDeployment deletes the deployment specify by the user
func (k *KubeClient) DeleteDeployment(
	name string,
	namespace string,
) (err error) {
	deployment := &appsv1.Deployment{}
	er := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		deployment,
	)
	if er == nil {
		err = k.Client.Delete(context.TODO(), deployment)
		if err != nil {
			return
		}
	}
	return
}

// DeletePVC deletes the PVC specify by the user
func (k *KubeClient) DeletePVC(
	name string,
	namespace string,
) (err error) {
	pvc := &corev1.PersistentVolumeClaim{}
	er := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		pvc,
	)
	if er == nil {
		err = k.Client.Delete(context.TODO(), pvc)
		if err != nil {
			return
		}
	}
	return
}

// DeleteConfigMap deletes the CM specify by the user
func (k *KubeClient) DeleteConfigMap(
	name string,
	namespace string,
) (err error) {
	cm := &corev1.ConfigMap{}
	er := k.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		cm,
	)
	if er == nil {
		err = k.Client.Delete(context.TODO(), cm)
		if err != nil {
			return
		}
	}
	return
}

// ConfigMapMutateFunc is the function for update k8s config maps gracefully
func ConfigMapMutateFunc(data map[string]string) controllerutil.MutateFn {
	return func(obj runtime.Object) error {
		configMap := obj.(*corev1.ConfigMap)
		configMap.Data = data
		return nil
	}
}

// PVCMutateFunc is the function for update k8s persistence volumes claims gracefully
func PVCMutateFunc(
	accessModes []corev1.PersistentVolumeAccessMode,
	storage string,
	class string,
) controllerutil.MutateFn {
	return func(obj runtime.Object) error {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Spec.AccessModes = accessModes
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(storage),
		}
		pvc.Spec.StorageClassName = &class
		return nil
	}
}

// ServiceMutateFunc is the function for update k8s services gracefully.
func ServiceMutateFunc(selectors map[string]string, servicePorts []corev1.ServicePort) controllerutil.MutateFn {
	return func(obj runtime.Object) error {
		service := obj.(*corev1.Service)
		service.Spec.Selector = selectors
		service.Spec.Ports = servicePorts
		return nil
	}
}

// DeploymentMutateFunc is the function for update k8s deployments gracefully
func DeploymentMutateFunc(
	replicas int32,
	labels map[string]string,
	image string,
	containerName string,
	command []string,
	args []string,
	ports []corev1.ContainerPort,
	volumeMounts []corev1.VolumeMount,
	envs []corev1.EnvVar,
	sc corev1.SecurityContext,
	ipp corev1.PullPolicy,
	secrets []corev1.LocalObjectReference,
	volumes []corev1.Volume,
) controllerutil.MutateFn {
	return func(obj runtime.Object) error {
		deployment := obj.(*appsv1.Deployment)
		deployment.Spec.Template.Spec = corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Image:           image,
					Name:            containerName,
					Command:         command,
					Args:            args,
					Ports:           ports,
					VolumeMounts:    volumeMounts,
					Env:             envs,
					SecurityContext: &sc,
					ImagePullPolicy: ipp,
				},
			},
			ImagePullSecrets: secrets,
			Volumes:          volumes,
		}
		return nil
	}
}

// IngressMutateFunc is the function for update k8s ingress gracefully
func IngressMutateFunc(
	namespace string,
	serviceName string,
	tls string,
	containerPorts []corev1.ContainerPort,
) controllerutil.MutateFn {
	return func(obj runtime.Object) error {
		ingress := obj.(*extensionsv1beta1.Ingress)
		var ingressPaths []extensionsv1beta1.HTTPIngressPath
		for _, port := range containerPorts {
			path := "/" + strings.ToLower(serviceName) +
				"/" + strconv.Itoa(int(port.ContainerPort)) + "(/|$)(.*)"
			ingressPath := extensionsv1beta1.HTTPIngressPath{
				Path: path,
				Backend: extensionsv1beta1.IngressBackend{
					ServiceName: strings.ToLower(serviceName),
					ServicePort: intstr.IntOrString{
						Type:   Int,
						IntVal: port.ContainerPort,
					},
				},
			}
			ingressPaths = append(ingressPaths, ingressPath)
		}
		currentRules := ingress.Spec.Rules
		ruleValue := extensionsv1beta1.IngressRuleValue{
			HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
				Paths: ingressPaths,
			},
		}
		newRule := extensionsv1beta1.IngressRule{
			Host:             IngressName,
			IngressRuleValue: ruleValue,
		}
		ruleExists := false
		for _, rule := range currentRules {
			if rule.Host == IngressName {
				ruleExists = true
				for _, path := range ingressPaths {
					if !pathContains(rule.HTTP.Paths, path) {
						rule.HTTP.Paths = append(rule.HTTP.Paths, path)
					}
				}
			}
		}
		if !ruleExists {
			currentRules = append(currentRules, newRule)
		}
		var ingressSpec extensionsv1beta1.IngressSpec
		if tls != "" {
			ingressSpec = extensionsv1beta1.IngressSpec{
				TLS: []extensionsv1beta1.IngressTLS{
					extensionsv1beta1.IngressTLS{
						Hosts:      []string{IngressName},
						SecretName: tls,
					},
				},
				Rules: currentRules,
			}
		} else {
			ingressSpec = extensionsv1beta1.IngressSpec{
				Rules: currentRules,
			}
		}
		ingress.Spec = ingressSpec
		return nil
	}
}

// pathContains checks the given path is available on ingress path lists or not
func pathContains(paths []extensionsv1beta1.HTTPIngressPath, path extensionsv1beta1.HTTPIngressPath) bool {
	for _, p := range paths {
		if reflect.DeepEqual(p, path) {
			return true
		}
	}
	return false
}
