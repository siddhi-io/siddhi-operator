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
	"testing"

	goctx "context"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

// failoverDeploymentTest check the failover deployment of a siddhi app.
// Check whether deployments, services, and PVCs created successfully
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
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-monitor-app-1", 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-monitor-app-2", 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().Services(namespace).Get("siddhi-nats", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().Services(namespace).Get("failover-monitor-app-2", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
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

// failoverConfigChangeTest check whether the configuration change of the siddhi runner happens correctly or not
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

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-monitor-app-1", 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-monitor-app-2", 1, retryInterval, timeout)
	if err != nil {
		return err
	}

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

// failoverPVCTest check whether PVC create correctly or not in the siddhi app failover deployment
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
			Name:      "failover-test-app",
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
					Class: "standard",
				},
			},
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-test-app-1", 1, retryInterval, mtimeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-test-app-2", 1, retryInterval, mtimeout)
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Get("failover-test-app-1-pvc", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	err = f.Client.Delete(goctx.TODO(), exampleSiddhi)
	if err != nil {
		return err
	}
	return nil
}
