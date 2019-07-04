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
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/siddhi-io/siddhi-operator/pkg/apis"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	mtimeout             = time.Second * 80
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

type MonitorRequest struct {
	Type     string `json:"type"`
	DeviceID string `json:"deviceID"`
	Power    int    `json:"power"`
}

func TestSiddhiProcess(t *testing.T) {
	siddhiList := &siddhiv1alpha2.SiddhiProcessList{
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

	if err = failoverDeploymentTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = failoverConfigChangeTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = failoverPVCTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}
