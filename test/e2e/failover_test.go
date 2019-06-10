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
	"errors"
	"strings"
	"testing"
	"time"

	goctx "context"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func failoverDeploymentTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	query := `@App:name("FMonitorApp")
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
			Name:      "test-monitor-app",
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
			DeploymentConfigs: siddhiv1alpha1.DeploymentConfigs{
				Mode: "failover",
			},
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	time.Sleep(40 * time.Second)

	_, err = f.KubeClient.CoreV1().Services(namespace).Get("siddhi-nats", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	depList, err := f.KubeClient.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	qExists := false
	pExists := false
	for _, dep := range depList.Items {
		if strings.HasPrefix(dep.GetName(), "fmonitorapp") {
			qExists = true
		}
		if strings.HasPrefix(dep.GetName(), "fmonitorapp-passthrough") {
			pExists = true
		}
	}
	if !qExists {
		err = errors.New("Query app deployment not found")
		return err
	}
	if !pExists {
		err = errors.New("Passthrough app deployment not found")
		return err
	}

	pSVCExists := false
	svcList, err := f.KubeClient.CoreV1().Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, svc := range svcList.Items {
		if strings.HasPrefix(svc.GetName(), "fmonitorapp-passthrough") {
			pSVCExists = true
		}
	}
	if !pSVCExists {
		err = errors.New("Passthrough app service not found")
		return err
	}

	_, err = f.KubeClient.ExtensionsV1beta1().Ingresses(namespace).Get("siddhi", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}

func failoverConfigChangeTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	query := `@App:name("FMonitorApp")
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
			Name:      "failover-monitor-app",
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
			DeploymentConfigs: siddhiv1alpha1.DeploymentConfigs{
				Mode: "failover",
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

	time.Sleep(30 * time.Second)

	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("failover-monitor-app-deployment.yaml", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}

func failoverPVCTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	query := `@App:name("FMonitorApp")
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
			Name:      "test-monitor-app",
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
			DeploymentConfigs: siddhiv1alpha1.DeploymentConfigs{
				Mode: "failover",
				PersistenceVolume: siddhiv1alpha1.PersistenceVolume{
					AccessModes: []string{
						"ReadWriteOnce",
					},
					VolumeMode: "Filesystem",
					Resources: siddhiv1alpha1.PVCResource{
						Requests: siddhiv1alpha1.PVCRequest{
							Storage: "1Gi",
						},
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

	time.Sleep(40 * time.Second)

	pvcList, err := f.KubeClient.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	qExists := false
	pExists := false
	for _, pvc := range pvcList.Items {
		if strings.HasPrefix(pvc.GetName(), "fmonitorapp") {
			qExists = true
		}
		if strings.HasPrefix(pvc.GetName(), "fmonitorapp-passthrough") {
			pExists = true
		}
	}
	if !qExists {
		err = errors.New("Query app deployment not found")
		return err
	}
	if !pExists {
		err = errors.New("Passthrough app deployment not found")
		return err
	}

	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}
