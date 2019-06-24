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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createPVC function creates a persistence volume claim for a K8s cluster
func (rsp *ReconcileSiddhiProcess) createPVC(sp *siddhiv1alpha1.SiddhiProcess, configs Configs, pvcName string) error {
	var accessModes []corev1.PersistentVolumeAccessMode
	pvc := &corev1.PersistentVolumeClaim{}
	p := sp.Spec.DeploymentConfigs.PersistenceVolume
	err := rsp.client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: sp.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		for _, am := range p.AccessModes {
			if am == configs.ReadWriteOnce {
				accessModes = append(accessModes, corev1.ReadWriteOnce)
				continue
			}
			if am == configs.ReadOnlyMany {
				accessModes = append(accessModes, corev1.ReadOnlyMany)
				continue
			}
			if am == configs.ReadWriteMany {
				accessModes = append(accessModes, corev1.ReadWriteMany)
				continue
			}
		}
		pvc = &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: sp.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: accessModes,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(p.Resources.Requests.Storage),
					},
				},
				StorageClassName: &p.Class,
			},
		}
		controllerutil.SetControllerReference(sp, pvc, rsp.scheme)
		err = rsp.client.Create(context.TODO(), pvc)
	}
	return err
}
