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
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	corev1 "k8s.io/api/core/v1"
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

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha2.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failover-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha2.SiddhiProcessSpec{
			Apps: []siddhiv1alpha2.Apps{
				siddhiv1alpha2.Apps{
					Script: script2,
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
			MessagingSystem: siddhiv1alpha2.MessagingSystem{
				Type: "nats",
			},
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-app-0", 1, retryInterval, maxTimeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-app-1", 1, retryInterval, maxTimeout)
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().Services(namespace).Get("siddhi-nats", metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().Services(namespace).Get("failover-app-0", metav1.GetOptions{IncludeUninitialized: true})
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

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha2.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failover-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha2.SiddhiProcessSpec{
			Apps: []siddhiv1alpha2.Apps{
				siddhiv1alpha2.Apps{
					Script: script2,
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
			MessagingSystem: siddhiv1alpha2.MessagingSystem{
				Type: "nats",
			},
			SiddhiConfig: StatePersistenceConf,
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-app-0", 1, retryInterval, maxTimeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-app-1", 1, retryInterval, maxTimeout)
	if err != nil {
		return err
	}

	_, err = f.KubeClient.CoreV1().ConfigMaps(namespace).Get("failover-app-depyml", metav1.GetOptions{IncludeUninitialized: true})
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

	// create SiddhiProcess custom resource
	exampleSiddhi := &siddhiv1alpha2.SiddhiProcess{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SiddhiProcess",
			APIVersion: "siddhi.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failover-test-app",
			Namespace: namespace,
		},
		Spec: siddhiv1alpha2.SiddhiProcessSpec{
			Apps: []siddhiv1alpha2.Apps{
				siddhiv1alpha2.Apps{
					Script: script2,
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
			MessagingSystem: siddhiv1alpha2.MessagingSystem{
				Type: "nats",
			},
			PV: siddhiv1alpha2.PV{
				AccessModes: []string{
					"ReadWriteOnce",
				},
				VolumeMode: "Filesystem",
				Resources: siddhiv1alpha2.PVCResource{
					Requests: siddhiv1alpha2.PVCRequest{
						Storage: "1Gi",
					},
				},
				Class: "standard",
			},
		},
	}

	err = f.Client.Create(goctx.TODO(), exampleSiddhi, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-test-app-0", 1, retryInterval, maxTimeout)
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "failover-test-app-1", 1, retryInterval, maxTimeout)
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
