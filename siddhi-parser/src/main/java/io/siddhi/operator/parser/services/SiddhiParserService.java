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
package io.siddhi.operator.parser.services;

import com.google.gson.Gson;
import io.siddhi.core.SiddhiAppRuntime;
import io.siddhi.core.SiddhiManager;
import io.siddhi.core.stream.ServiceDeploymentInfo;
import io.siddhi.core.stream.input.source.Source;
import io.siddhi.core.util.config.InMemoryConfigManager;
import io.siddhi.operator.parser.bean.AppDeploymentInfo;
import io.siddhi.operator.parser.bean.SiddhiParserRequest;
import io.siddhi.operator.parser.bean.SiddhiAppConfig;
import io.siddhi.operator.parser.bean.SourceDeploymentConfig;
import io.siddhi.operator.parser.bean.SourceList;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import javax.ws.rs.Consumes;
import javax.ws.rs.GET;
import javax.ws.rs.NotFoundException;
import javax.ws.rs.POST;
import javax.ws.rs.Path;
import javax.ws.rs.Produces;
import javax.ws.rs.core.MediaType;
import javax.ws.rs.core.Response;

/**
 * Siddhi Parser Service used by the Kubernetes Operator to parse the Siddhi Apps
 */
@Path("/siddhi-parser")
public class SiddhiParserService {

    private static final String CARBON_HOME = "carbon.home";
    private static final SiddhiManager siddhimanager = new SiddhiManager();
    private static final Logger log = LoggerFactory.getLogger(SiddhiParserService.class);

    SiddhiParserService() {
        setCarbonHome();
        Map<String, String> masterConfigs = new HashMap<>();
        masterConfigs.put("source.http.keyStoreLocation", "${carbon.home}/resources/security/wso2carbon.jks");
        masterConfigs.put("source.http.keyStorePassword", "wso2carbon");
        masterConfigs.put("source.http.certPassword", "wso2carbon");
        InMemoryConfigManager inMemoryConfigManager = new InMemoryConfigManager(masterConfigs, null);
        inMemoryConfigManager.generateConfigReader("source", "http");
        siddhimanager.setConfigManager(inMemoryConfigManager);
    }

    @GET
    @Path("/")
    public String get() {
        return "Siddhi Parser Service is up and running.";
    }

    @POST
    @Path("/parse")
    @Consumes({"application/json"})
    @Produces({"application/json"})
    public Response query(SiddhiParserRequest request)
            throws NotFoundException {
        try {
            List<String> siddhiApps = populateEnvs(request.getPropertyMap(), request.getSiddhiApp());
            AppDeploymentInfo appDeploymentInfo = getExposedDeploymentConfigs(siddhiApps);
            String jsonString = new Gson().toJson(appDeploymentInfo);
            return Response.ok(jsonString, MediaType.APPLICATION_JSON)
                    .build();
        } catch (Exception e) {
            log.error("Failed to parse Siddhi app. ", e);
            return Response.serverError().entity("Failed passing the Siddhi apps").build();
        }
    }

    private static List<String> populateEnvs(Map<String, String> envMap, List<String> siddhiApps) {
        List<String> populatedApps = new ArrayList<>();
        for (String siddhiApp : siddhiApps) {
            if (siddhiApp.contains("$")) {
                String envPattern = "\\$\\{(\\w+)\\}";
                Pattern expr = Pattern.compile(envPattern);
                Matcher matcher = expr.matcher(siddhiApp);
                while (matcher.find()) {
                    for (int i = 1; i <= matcher.groupCount(); i++) {
                        String envValue = envMap.get(matcher.group(i).toUpperCase());
                        if (envValue == null) {
                            envValue = "";
                        } else {
                            envValue = envValue.replace("\\", "\\\\");
                        }
                        Pattern subexpr = Pattern.compile("\\$\\{" + matcher.group(i) + "\\}");
                        siddhiApp = subexpr.matcher(siddhiApp).replaceAll(envValue);
                    }
                }
            }
            populatedApps.add(siddhiApp);
        }
        return populatedApps;
    }

    private static AppDeploymentInfo getExposedDeploymentConfigs(List<String> siddhiApps) {
        AppDeploymentInfo appDeploymentInfo = new AppDeploymentInfo();
        for (String siddhiApp : siddhiApps) {
            SiddhiAppConfig siddhiAppConfig = new SiddhiAppConfig();
            siddhiAppConfig.setSiddhiApp(siddhiApp);
            SiddhiAppRuntime siddhiAppRuntime = siddhimanager.createSiddhiAppRuntime(siddhiApp);
            SourceList sourceConfigs = new SourceList();
            Collection<List<Source>> sources = siddhiAppRuntime.getSources();
            for (List<Source> sourceList : sources) {
                for (Source source : sourceList) {
                    if (source.getType().equalsIgnoreCase("http")) {
                        SourceDeploymentConfig response = new SourceDeploymentConfig();
                        ServiceDeploymentInfo serviceDeploymentInfo = source.getServiceDeploymentInfo();
                        response.setPort(serviceDeploymentInfo.getPort());
                        response.setServiceProtocol(serviceDeploymentInfo.getServiceProtocol().name());
                        response.setSecured(serviceDeploymentInfo.isSecured());
                        response.setDeploymentProperties(serviceDeploymentInfo.getDeploymentProperties());
                        sourceConfigs.addSourceDeploymentConfig(response);
                    }
                }
            }
            siddhiAppConfig.setSourceList(sourceConfigs);
            appDeploymentInfo.addSiddhiAppConfigs(siddhiAppConfig);
        }
        return appDeploymentInfo;
    }

    private static void setCarbonHome() {
        java.nio.file.Path carbonHome = Paths.get("");
        carbonHome = Paths.get(carbonHome.toString(), "src", "test");
        System.setProperty(CARBON_HOME, carbonHome.toString());
    }
}
