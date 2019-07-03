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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	// "sigs.k8s.io/controller-runtime/pkg/reconcile"
	// "k8s.io/apimachinery/pkg/types"
)

var app1 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

var app2 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

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
    sp.Spec = spec
	configs := getTestConfigs(sp)
	propertyMap := map[string]string{}
	siddhiParserRequest := populateParserRequest(sp, siddhiApps, propertyMap, configs)
	if siddhiParserRequest.MessagingSystem.EmptyConfig() {
		t.Error("Empty messaging system in SiddhiParserRequest")
	}
}

func TestPopulateRunnerConfigs(t *testing.T) {
	image := "siddhiio/siddhi-runner:0.1.1"
	secret := "siddhiio-secret"
    spec := siddhiv1alpha2.SiddhiProcessSpec{
        Container: corev1.Container{
            Image: image,
        },
        ImagePullSecret: secret,
    }
    sp.Spec = spec
	configs := getTestConfigs(sp)
	img, home, secrt := populateRunnerConfigs(sp, configs)
	if img != image {
		t.Error("Expected image name " + image + " but found " + img)
	}
	if img != image {
		t.Error("Expected home name " + configs.SiddhiHome + " but found " + home)
	}
	if img != image {
		t.Error("Expected imagePullSecret " + secret + " but found " + secrt)
	}

}

func TestPopulateMountPath(t *testing.T) {
    spec := siddhiv1alpha2.SiddhiProcessSpec{
        SiddhiConfig: StatePersistenceConf,
    }
    sp.Spec = spec
	configs := getTestConfigs(sp)
	path := configs.SiddhiHome + configs.SiddhiRunnerPath + "siddhi-app-persistence"
	p, err := populateMountPath(sp, configs)
	if err != nil {
		t.Error(err)
	}
	if p != path {
		t.Error("Expected mount path " + path + " but found " + p)
	}

	sp.Spec.SiddhiConfig = StatePersistenceConfTest
	path = "/home/siddhi"
	p, err = populateMountPath(sp, configs)
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

func getTestConfigs(sp *siddhiv1alpha2.SiddhiProcess) (configs Configs) {
	s := scheme.Scheme
	s.AddKnownTypes(siddhiv1alpha2.SchemeGroupVersion, sp)
	objs := []runtime.Object{sp}
	cl := fake.NewFakeClient(objs...)
	rsp := &ReconcileSiddhiProcess{client: cl, scheme: s}
	configs = rsp.Configurations(sp)
	return
}
