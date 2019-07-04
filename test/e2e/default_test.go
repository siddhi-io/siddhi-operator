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

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	goctx "context"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

var script1 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@sink(type='log', prefix='LOGGER')
@source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
define stream DevicePowerStream (type string, deviceID string, power int);

define stream MonitorDevicesPowerStream(deviceID string, power int);
@info(name='monitored-filter')
from DevicePowerStream[type == 'monitored']
select deviceID, power
insert into MonitorDevicesPowerStream;`

var script2 = `@App:name("MonitorApp")
@App:description("Description of the plan") 

@source(
    type='http',
    receiver.url='http://0.0.0.0:8080/example',
    basic.auth.enabled='false',
    @map(type='json')
)
define stream DevicePowerStream (type string, deviceID string, power int);

@sink(type='log', prefix='LOGGER')
define stream MonitorDevicesPowerStream(sumPower long);
@info(name='monitored-filter')
from DevicePowerStream#window.time(100 min)
select sum(power) as sumPower
insert all events into MonitorDevicesPowerStream;`

// siddhiDeploymentTest test the default deployment of a siddhi app
// Check whether deployment, service, ingress, and config map of the siddhi app deployment created correctly
func siddhiDeploymentTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha2.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha2.SiddhiProcessSpec{
			Apps: []siddhiv1alpha2.Apps{
				siddhiv1alpha2.Apps{
					Script: script1,
				},
			},
			Container: corev1.Container{
				Env: []corev1.EnvVar{
					corev1.EnvVar{
						Name:  "RECEIVER_URL",
						Value: "http://0.0.0.0:8280/example",
					},
					corev1.EnvVar{
						Name:  "BASIC_AUTH_ENABLED",
						Value: "false",
					},
				},
			},
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "test-monitor-app-0", 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().Services(namespace).Get("test-monitor-app-0", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	_, err = f.KubeClient.ExtensionsV1beta1().Ingresses(namespace).Get("siddhi", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("test-monitor-app-0-siddhi", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	reqObj := MonitorRequest{
		Type:     "monitored",
		DeviceID: "001",
		Power:    343,
	}
	b, err := json.Marshal(reqObj)
	if err != nil {
		t.Errorf("JSON marshan error")
		return err
	}
	url := "http://siddhi/test-monitor-app-0/8280/example"
	var jsonStr = []byte(string(b))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("REST invoking error %s", url)
		return err
	}
	defer resp.Body.Close()

	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}

// siddhiConfigChangeTest check whether the configuration change of the siddhi runner deployment.yaml
// correctly executed or not. Here mainly it checks the config map creation was successfull or not.
func siddhiConfigChangeTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha2.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha2.SiddhiProcessSpec{
			Apps: []siddhiv1alpha2.Apps{
				siddhiv1alpha2.Apps{
					Script: script1,
				},
			},
			Container: corev1.Container{
				Env: []corev1.EnvVar{
					corev1.EnvVar{
						Name:  "RECEIVER_URL",
						Value: "http://0.0.0.0:8280/example",
					},
					corev1.EnvVar{
						Name:  "BASIC_AUTH_ENABLED",
						Value: "false",
					},
				},
			},
			SiddhiConfig: `
				state.persistence:
					enabled: true
					intervalInMin: 5
					revisionsToKeep: 2
					persistenceStore: io.siddhi.distribution.core.persistence.FileSystemPersistenceStore
					config:
						location: siddhi-app-persistence`,
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "test-monitor-app-0", 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("test-monitor-app-deployment-yaml", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}
