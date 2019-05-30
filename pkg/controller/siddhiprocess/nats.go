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
	 siddhiv1alpha1 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha1"
	 
	 "k8s.io/apimachinery/pkg/api/errors"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 natsv1alpha2 "github.com/nats-io/nats-operator/pkg/apis/nats/v1alpha2"
	 streamingv1alpha1 "github.com/nats-io/nats-streaming-operator/pkg/apis/streaming/v1alpha1"
	 "k8s.io/apimachinery/pkg/types"
	 "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
 )

 func (rsp *ReconcileSiddhiProcess) createNATS(sp *siddhiv1alpha1.SiddhiProcess, configs Configs) error {
	 natsCluster := natsv1alpha2.NatsCluster{}
	 natsName := sp.Name + configs.NATSExt
	 err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: natsName, Namespace: sp.Namespace}, natsCluster)
	 if err != nil && errors.IsNotFound(err) {
		 natsCluster = natsv1alpha2.NatsCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configs.NATSAPIVersion,
				Kind:       configs.NATSKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      natsName,
				Namespace: sp.Namespace,
			},
			Spec: natsv1alpha2.ClusterSpec{
				Size: configs.NATSSize,
			},
		 }
		 controllerutil.SetControllerReference(sp, natsCluster, rsp.scheme)
		 err = rsp.client.Create(context.TODO(), natsCluster)
		 if err != nil {
			 return err
		 }
	 }
	 stanCluster := streamingv1alpha1.NatsStreamingCluster{}
	 stanName := sp.Name + configs.STANExt
	 err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: stanName, Namespace: sp.Namespace}, stanCluster)
	 if err != nil && errors.IsNotFound(err) {
		stanCluster = streamingv1alpha1.NatsStreamingCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configs.STANAPIVersion,
				Kind:       configs.STANKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      stanName,
				Namespace: sp.Namespace,
			},
			Spec: streamingv1alpha1.NatsStreamingClusterSpec{
				Size: configs.Size,
				NatsService: natsName,
			},
		 }
		 controllerutil.SetControllerReference(sp, stanCluster, rsp.scheme)
		 err = rsp.client.Create(context.TODO(), stanCluster)
		 if err != nil {
			return err
		}
	 }
	 return err
 }
 