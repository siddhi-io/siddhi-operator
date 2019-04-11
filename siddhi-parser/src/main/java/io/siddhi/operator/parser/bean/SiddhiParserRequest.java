/*
 * Copyright (c) 2019, WSO2 Inc. (http://wso2.com) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package io.siddhi.operator.parser.bean;

import com.google.gson.annotations.SerializedName;

import java.util.List;
import java.util.Map;

public class SiddhiParserRequest {

    @SerializedName("siddhiApps")
    private List<String> siddhiApps = null;

    @SerializedName("propertyMap")
    private Map<String, String> propertyMap = null;

    public List<String> getSiddhiApp() {
        return siddhiApps;
    }

    public SiddhiParserRequest setSiddhiApp(List<String> siddhiApp) {
        this.siddhiApps = siddhiApp;
        return this;
    }

    public Map<String, String> getPropertyMap() {
        return propertyMap;
    }

    public SiddhiParserRequest setPropertyMap(Map<String, String> propertyMap) {
        this.propertyMap = propertyMap;
        return this;
    }
}
