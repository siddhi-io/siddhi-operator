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

package messaging

import (
	"errors"

	siddhiv1alpha2 "github.com/siddhi-io/siddhi-operator/pkg/apis/siddhi/v1alpha2"
	artifact "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/artifact"
	deploymanager "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess/deploymanager"
)

// Messaging creates a specific messaging system like NATS
type Messaging struct {
	KubeClient    artifact.KubeClient
	SiddhiProcess *siddhiv1alpha2.SiddhiProcess
}

// CreateMessagingSystem creates the messaging system if CR needed.
// If user specify only the messaging system type then this will creates the messaging system.
func (m *Messaging) CreateMessagingSystem(
	applications []deploymanager.Application,
) (err error) {
	persistenceEnabled := false
	for _, application := range applications {
		if application.PersistenceEnabled {
			persistenceEnabled = true
			break
		}
	}
	if m.SiddhiProcess.Spec.MessagingSystem.TypeDefined() && m.SiddhiProcess.Spec.MessagingSystem.EmptyConfig() && persistenceEnabled {
		if !m.CheckMessagingSystem() {
			return errors.New("Automatic NATS creation fails. NATS not configured in the cluster")
		}
		err = m.KubeClient.CreateNATS(m.SiddhiProcess.Namespace)
		if err != nil {
			return
		}
	}
	return
}

// CheckMessagingSystem checks the availability of the NATS operator and
// NATS streaming operator. Then it returns true if both are enabled, otherwise return false.
func (m *Messaging) CheckMessagingSystem() bool {
	_, isNatsAvailable := m.KubeClient.GetDeployment(NATSOperatorName, m.SiddhiProcess.Namespace)
	_, isStanAvailable := m.KubeClient.GetDeployment(STANOperatorName, m.SiddhiProcess.Namespace)

	if isNatsAvailable && isStanAvailable {
		return true
	}
	return false
}
