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
	"strconv"
	"testing"

	natsv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/nats/v1alpha2"
	streamingv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/streaming/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var kubeClient = KubeClient{
	Scheme: scheme.Scheme,
	Client: fake.NewFakeClient([]runtime.Object{}...),
}

// apps to run tests
var sampleText = `
It was popularised in the 1960s.
`

var namespace = "default"

var sampleDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sample",
		Namespace: namespace,
	},
}

func TestCreateOrUpdateCM(t *testing.T) {
	name := "monitor-app"
	data := map[string]string{
		"SampleData": sampleText,
	}
	err := kubeClient.CreateOrUpdateCM(name, namespace, data, sampleDeployment)
	if err != nil {
		t.Error(err)
	}
	configMap := &corev1.ConfigMap{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, configMap)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateOrUpdateIngress(t *testing.T) {
	serviceName := "sampleService"
	containerPorts := []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          "samplePort1",
			ContainerPort: 8080,
		},
	}
	_, err := kubeClient.CreateOrUpdateIngress(
		namespace,
		serviceName,
		"",
		containerPorts,
	)
	if err != nil {
		t.Error(err)
	}
	ingress := &extensionsv1beta1.Ingress{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: IngressName, Namespace: namespace}, ingress)
	if err != nil {
		t.Error(err)
	}

	containerPorts = []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          "samplePort2",
			ContainerPort: 8081,
		},
	}
	_, err = kubeClient.CreateOrUpdateIngress(
		namespace,
		serviceName,
		"",
		containerPorts,
	)
	if err != nil {
		t.Error(err)
	}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: IngressName, Namespace: namespace}, ingress)
	if err != nil {
		t.Error(err)
	}
	if len(ingress.Spec.Rules[0].HTTP.Paths) != 2 {
		t.Error("Ingress update error. Expected entries 2, but found " + strconv.Itoa(len(ingress.Spec.Rules)))
	}
}

func TestCreateNATS(t *testing.T) {
	kubeClient.Scheme.AddKnownTypes(natsv1alpha2.SchemeGroupVersion)
	kubeClient.Scheme.AddKnownTypes(streamingv1alpha1.SchemeGroupVersion)
	err := natsv1alpha2.AddToScheme(kubeClient.Scheme)
	if err != nil {
		t.Error(err)
	}
	err = streamingv1alpha1.AddToScheme(kubeClient.Scheme)
	if err != nil {
		t.Error(err)
	}
	if err != nil {
		t.Error(err)
	}
	err = kubeClient.CreateNATS(namespace)
	natsCluster := &natsv1alpha2.NatsCluster{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: NATSClusterName, Namespace: namespace}, natsCluster)
	if err != nil {
		t.Error(err)
	}

	stanCluster := &streamingv1alpha1.NatsStreamingCluster{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: STANClusterName, Namespace: namespace}, stanCluster)
	if err != nil {
		t.Error(err)
	}

}

func TestCreateOrUpdatePVC(t *testing.T) {

	name := "sample-pvc"
	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
		corev1.ReadWriteMany,
	}
	storageClassName := "standard"
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes:      accessModes,
		StorageClassName: &storageClassName,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(1*1024*1024*1024, resource.BinarySI),
			},
		},
	}
	err := kubeClient.CreateOrUpdatePVC(name, namespace, pvcSpec, sampleDeployment)
	if err != nil {
		t.Error(err)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, pvc)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateOrUpdateService(t *testing.T) {
	name := "sampleService"
	containerPorts := []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          "samplePort1",
			ContainerPort: 8080,
		},
	}
	selectors := map[string]string{
		"app":     "sample",
		"version": "0.1.0",
	}
	_, err := kubeClient.CreateOrUpdateService(
		name,
		namespace,
		containerPorts,
		selectors,
		sampleDeployment,
	)
	if err != nil {
		t.Error(err)
	}
	service := &corev1.Service{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, service)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateOrUpdateDeployment(t *testing.T) {
	name := "sampleDeployment"
	containerPorts := []corev1.ContainerPort{
		corev1.ContainerPort{
			Name:          "samplePort1",
			ContainerPort: 8080,
		},
	}
	selectors := map[string]string{
		"app":     "sample",
		"version": "0.1.0",
	}
	image := "siddhiio/siddhi-operator:0.2.1"
	_, err := kubeClient.CreateOrUpdateDeployment(
		name,
		namespace,
		1,
		selectors,
		image,
		"siddhirunner",
		[]string{"sh"},
		[]string{"-Dsiddhi-parser"},
		containerPorts,
		[]corev1.VolumeMount{},
		[]corev1.EnvVar{},
		corev1.SecurityContext{},
		corev1.PullAlways,
		[]corev1.LocalObjectReference{},
		[]corev1.Volume{},
		appsv1.DeploymentStrategy{},
		sampleDeployment,
	)
	if err != nil {
		t.Error(err)
	}
	deployment := &appsv1.Deployment{}
	err = kubeClient.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, deployment)
	if err != nil {
		t.Error(err)
	}
}
