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

package unit

import (
	"testing"
	sp "github.com/siddhi-io/siddhi-operator/pkg/controller/siddhiprocess"
)

func TestGetAppName(t *testing.T) {
	app := `@App:name("MonitorApp")
    @App:description("Description of the plan") 
    
    @sink(type='log', prefix='LOGGER')
    @source(type='http', receiver.url='${RECEIVER_URL}', basic.auth.enabled='${BASIC_AUTH_ENABLED}', @map(type='json'))
    define stream DevicePowerStream (type string, deviceID string, power int);
    
    define stream MonitorDevicesPowerStream(deviceID string, power int);
    @info(name='monitored-filter')
    from DevicePowerStream[type == 'monitored']
    select deviceID, power
    insert into MonitorDevicesPowerStream;`

    appName := sp.GetAppName(app)
    if appName != "MonitorApp" {
        t.Error("GetAppName function fails. Expected but found ")
    } 
}