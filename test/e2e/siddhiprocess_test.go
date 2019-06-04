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
	"time"

	goctx "context"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/siddhi-io/siddhi-operator/pkg/apis"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

type MonitorRequest struct {
	Type     string `json:"type"`
	DeviceID string `json:"deviceID"`
	Power    int    `json:"power"`
}

func TestSiddhiProcess(t *testing.T) {
	siddhiList := &siddhiv1alpha1.SiddhiProcessList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
	}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, siddhiList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	t.Run("siddhi-group", func(t *testing.T) {
		t.Run("Cluster", SiddhiCluster)
	})
}

func siddhiDeploymentTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	query := `@App:name("MonitorApp")
    @App:description("Description of the plan") 
    
    @sink(type='log', prefix='LOGGER')
    @source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
    define stream DevicePowerStream (type string, deviceID string, power int);
    
    define stream MonitorDevicesPowerStream(deviceID string, power int);
    @info(name='monitored-filter')
    from DevicePowerStream[type == 'monitored']
    select deviceID, power
	insert into MonitorDevicesPowerStream;`

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha1.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha1.SiddhiProcessSpec{
			Query: query,
			EnviromentVariables: []siddhiv1alpha1.EnviromentVariable{
				siddhiv1alpha1.EnviromentVariable{
					Name:  "RECEIVER_URL",
					Value: "http://0.0.0.0:8280/example",
				},
				siddhiv1alpha1.EnviromentVariable{
					Name:  "BASIC_AUTH_ENABLED",
					Value: "false",
				},
			},
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "monitor-app", 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().Services(namespace).Get("monitor-app", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	_, err = f.KubeClient.ExtensionsV1beta1().Ingresses(namespace).Get("siddhi", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("monitor-app-siddhi", metav1.GetOptions{IncludeUninitialized: true})
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
	url := "http://siddhi/monitor-app/8280/example"
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
	return nil
}

func siddhiConfigChangeTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	query := `@App:name("MonitorApp")
    @App:description("Description of the plan") 
    
    @sink(type='log', prefix='LOGGER')
    @source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
    define stream DevicePowerStream (type string, deviceID string, power int);
    
    define stream MonitorDevicesPowerStream(deviceID string, power int);
    @info(name='monitored-filter')
    from DevicePowerStream[type == 'monitored']
    select deviceID, power
	insert into MonitorDevicesPowerStream;`

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha1.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-app1",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha1.SiddhiProcessSpec{
			Query: query,
			EnviromentVariables: []siddhiv1alpha1.EnviromentVariable{
				siddhiv1alpha1.EnviromentVariable{
					Name:  "RECEIVER_URL",
					Value: "http://0.0.0.0:8280/example",
				},
				siddhiv1alpha1.EnviromentVariable{
					Name:  "BASIC_AUTH_ENABLED",
					Value: "false",
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
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "monitor-app1", 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("monitor-app1-deployment.yaml", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	return nil
}

func SiddhiCluster(t *testing.T) {
	t.Parallel()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for siddhi-operator to be ready
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "siddhi-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	if err = siddhiDeploymentTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = siddhiConfigChangeTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}
