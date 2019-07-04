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
	"testing"

	natsv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/nats/v1alpha2"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	streamingv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/streaming/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateConfigMap(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	data := map[string]string{
		"MonitorApp": app,
	}
	configMapName := "siddhiApp"
	err := rsp.CreateConfigMap(testSP, configMapName, data)
	if err != nil {
		t.Error(err)
	}
	configMap := &corev1.ConfigMap{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: testSP.Namespace}, configMap)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateAndUpdateIngress(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs := getTestConfigs(testSP)
	err := rsp.CreateIngress(testSP, testSiddhiApp, configs)
	if err != nil {
		t.Error(err)
	}
	ingress := &extensionsv1beta1.Ingress{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.HostName, Namespace: testSP.Namespace}, ingress)
	if err != nil {
		t.Error(err)
	}
	sa := SiddhiApp{
		Name: "MonitorApp",
		ContainerPorts: []corev1.ContainerPort{
			corev1.ContainerPort{
				Name:          "monitorapp8081",
				ContainerPort: 8081,
			},
		},
		Apps: map[string]string{
			"MonitorApp": app,
		},
		PersistenceEnabled: true,
	}
	err = rsp.UpdateIngress(testSP, ingress, sa, configs)
	if err != nil {
		t.Error(err)
	}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.HostName, Namespace: testSP.Namespace}, ingress)
	if err != nil {
		t.Error(err)
	}
	if len(ingress.Spec.Rules) != 2 {
		t.Error("Ingress update error. Expected entries 2, but found " + strconv.Itoa(len(ingress.Spec.Rules)))
	}
}

func TestCreateNATS(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	err := natsv1alpha2.AddToScheme(s)
	if err != nil {
		t.Error(err)
	}
	err = streamingv1alpha1.AddToScheme(s)
	if err != nil {
		t.Error(err)
	}
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs := getTestConfigs(testSP)
	err = rsp.CreateNATS(testSP, configs)
	if err != nil {
		t.Error(err)
	}

	natsCluster := &natsv1alpha2.NatsCluster{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.NATSClusterName, Namespace: testSP.Namespace}, natsCluster)
	if err != nil {
		t.Error(err)
	}

	stanCluster := &streamingv1alpha1.NatsStreamingCluster{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: configs.STANClusterName, Namespace: testSP.Namespace}, stanCluster)
	if err != nil {
		t.Error(err)
	}

}

func TestCreatePVC(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	pvcName := "monitorapp-pvc"
	configs := getTestConfigs(testSP)
	err := rsp.CreatePVC(testSP, configs, pvcName)
	if err != nil {
		t.Error(err)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: testSP.Namespace}, pvc)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateService(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs := getTestConfigs(testSP)
	err := rsp.CreateService(testSP, testSiddhiApp, configs)
	if err != nil {
		t.Error(err)
	}
	service := &corev1.Service{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: testSiddhiApp.Name, Namespace: testSP.Namespace}, service)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateDeployment(t *testing.T) {
	objs := []runtime.Object{testSP}
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs := getTestConfigs(testSP)
	labels := map[string]string{
		"appName": "monitorapp",
	}
	err := rsp.CreateDeployment(
		testSP,
		testSiddhiApp.Name,
		testSP.Namespace,
		1,
		labels,
		configs.SiddhiImage,
		"siddhirunner",
		[]string{configs.Shell},
		[]string{configs.SiddhiHome + configs.RunnerRPath},
		testSiddhiApp.ContainerPorts,
		[]corev1.VolumeMount{},
		[]corev1.EnvVar{},
		corev1.SecurityContext{},
		corev1.PullAlways,
		[]corev1.LocalObjectReference{},
		[]corev1.Volume{},
	)
	if err != nil {
		t.Error(err)
	}
	deployment := &appsv1.Deployment{}
	err = rsp.client.Get(context.TODO(), types.NamespacedName{Name: testSiddhiApp.Name, Namespace: testSP.Namespace}, deployment)
	if err != nil {
		t.Error(err)
	}
}
