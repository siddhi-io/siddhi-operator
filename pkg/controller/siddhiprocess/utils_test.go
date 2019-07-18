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
	"testing"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetAppName(t *testing.T) {
	appName, err := getAppName(app1)
	if err != nil {
		t.Error(err)
	}
	if appName != "MonitorApp" {
		t.Error("GetAppName function fails. Expect MonitorApp but found " + appName)
	}
}

func TestPopulateParserRequest(t *testing.T) {
	siddhiApps := []string{app1, app2}
	spec := siddhiv1alpha2.SiddhiProcessSpec{
		MessagingSystem: siddhiv1alpha2.MessagingSystem{
			Type: "nats",
		},
	}
	testSP.Spec = spec
	configs := getTestConfigs(testSP)
	propertyMap := map[string]string{}
	siddhiParserRequest := populateParserRequest(testSP, siddhiApps, propertyMap, configs)
	if siddhiParserRequest.MessagingSystem.EmptyConfig() {
		t.Error("Empty messaging system in SiddhiParserRequest")
	}
}

func TestPopulateRunnerConfigs(t *testing.T) {
	image := "siddhiio/siddhi-runner:5.1.0"
	secret := "siddhiio-secret"
	spec := siddhiv1alpha2.SiddhiProcessSpec{
		Container: corev1.Container{
			Image: image,
		},
		ImagePullSecret: secret,
	}
	testSP.Spec = spec
	configs := getTestConfigs(testSP)
	img, home, secrt := populateRunnerConfigs(testSP, configs)
	if img != image {
		t.Error("Expected image name " + image + " but found " + img)
	}
	if home != configs.SiddhiHome {
		t.Error("Expected home name " + configs.SiddhiHome + " but found " + home)
	}
	if secrt != secret {
		t.Error("Expected imagePullSecret " + secret + " but found " + secrt)
	}

}

func TestPopulateMountPath(t *testing.T) {
	spec := siddhiv1alpha2.SiddhiProcessSpec{
		SiddhiConfig: StatePersistenceConf,
	}
	testSP.Spec = spec
	configs := getTestConfigs(testSP)
	path := configs.SiddhiHome + configs.WSO2Dir + "/" + configs.SiddhiProfile + "/" + configs.FilePersistentDir
	p, err := populateMountPath(testSP, configs)
	if err != nil {
		t.Error(err)
	}
	if p != path {
		t.Error("Expected mount path " + path + " but found " + p)
	}

	testSP.Spec.SiddhiConfig = StatePersistenceConfTest
	path = "/home/siddhi"
	p, err = populateMountPath(testSP, configs)
	if err != nil {
		t.Error(err)
	}
	if p != path {
		t.Error("Expected mount path " + path + " but found " + p)
	}
}

func TestCreateCMVolumes(t *testing.T) {
	configMapName := "siddhi-cm"
	mountPath := "/home/siddhi"
	volume, volumeMount := createCMVolumes(configMapName, mountPath)
	if volume.Name != configMapName {
		t.Error("Expected volume name " + configMapName + " but found " + volume.Name)
	}
	if volume.VolumeSource.ConfigMap.LocalObjectReference.Name != configMapName {
		t.Error("Expected volume source name " + configMapName + " but found " + volume.VolumeSource.ConfigMap.LocalObjectReference.Name)
	}
	if volumeMount.Name != configMapName {
		t.Error("Expected volume mount name " + configMapName + " but found " + volumeMount.Name)
	}
	if volumeMount.MountPath != mountPath {
		t.Error("Expected volume mount name " + mountPath + " but found " + volumeMount.MountPath)
	}
}

func TestCreatePVCVolumes(t *testing.T) {
	pvcName := "siddhi-pvc"
	mountPath := "/home/siddhi"
	volume, volumeMount := createCMVolumes(pvcName, mountPath)
	if volume.Name != pvcName {
		t.Error("Expected volume name " + pvcName + " but found " + volume.Name)
	}
	if volume.VolumeSource.ConfigMap.LocalObjectReference.Name != pvcName {
		t.Error("Expected volume source name " + pvcName + " but found " + volume.VolumeSource.ConfigMap.LocalObjectReference.Name)
	}
	if volumeMount.Name != pvcName {
		t.Error("Expected volume mount name " + pvcName + " but found " + volumeMount.Name)
	}
	if volumeMount.MountPath != mountPath {
		t.Error("Expected volume mount name " + mountPath + " but found " + volumeMount.MountPath)
	}
}

func TestPathContains(t *testing.T) {
	paths := []extensionsv1beta1.HTTPIngressPath{
		extensionsv1beta1.HTTPIngressPath{
			Path: "/app1/8080/example",
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: "app1",
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: 8080,
				},
			},
		},
		extensionsv1beta1.HTTPIngressPath{
			Path: "/app2/8080/example",
			Backend: extensionsv1beta1.IngressBackend{
				ServiceName: "app2",
				ServicePort: intstr.IntOrString{
					Type:   Int,
					IntVal: 8080,
				},
			},
		},
	}
	negativePath := extensionsv1beta1.HTTPIngressPath{
		Path: "/app3/8080/example",
		Backend: extensionsv1beta1.IngressBackend{
			ServiceName: "app3",
			ServicePort: intstr.IntOrString{
				Type:   Int,
				IntVal: 8080,
			},
		},
	}
	positivePath := extensionsv1beta1.HTTPIngressPath{
		Path: "/app2/8080/example",
		Backend: extensionsv1beta1.IngressBackend{
			ServiceName: "app2",
			ServicePort: intstr.IntOrString{
				Type:   Int,
				IntVal: 8080,
			},
		},
	}

	if pathContains(paths, negativePath) {
		t.Error("Expected value pathContains=false but found true")
	}
	if !pathContains(paths, positivePath) {
		t.Error("Expected value pathContains=true but found false")
	}
}

func getTestConfigs(testSP *siddhiv1alpha2.SiddhiProcess) (configs Configs) {
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, testSP)
	objs := []runtime.Object{testSP}
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs = rsp.Configurations(testSP)
	return
}
