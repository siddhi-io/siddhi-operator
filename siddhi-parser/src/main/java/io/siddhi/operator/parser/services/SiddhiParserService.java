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
import io.siddhi.operator.app.builder.core.SiddhiAppCreator;
import io.siddhi.operator.app.builder.core.SiddhiTopologyCreator;
import io.siddhi.operator.app.builder.core.appcreator.DeployableSiddhiQueryGroup;
import io.siddhi.operator.app.builder.core.appcreator.NatsSiddhiAppCreator;
import io.siddhi.operator.app.builder.core.appcreator.SiddhiQuery;
import io.siddhi.operator.app.builder.core.topology.SiddhiTopology;
import io.siddhi.operator.app.builder.core.topology.SiddhiTopologyCreatorImpl;
import io.siddhi.operator.parser.bean.DeployableSiddhiApp;
import io.siddhi.operator.parser.bean.SiddhiParserRequest;
import io.siddhi.operator.parser.bean.SourceDeploymentConfig;
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
 * Siddhi Parser Service used by the Siddhi Kubernetes Operator to parse the Siddhi Apps
 */
@Path("/siddhi-parser")
public class SiddhiParserService {

    private static final String CARBON_HOME = "carbon.home";
    private SiddhiManager siddhimanager;
    private static final Logger log = LoggerFactory.getLogger(SiddhiParserService.class);

    SiddhiParserService() {
        setCarbonHome();
        Map<String, String> masterConfigs = new HashMap<>();
        masterConfigs.put("source.http.keyStoreLocation", "${carbon.home}/resources/security/wso2carbon.jks");
        masterConfigs.put("source.http.keyStorePassword", "wso2carbon");
        masterConfigs.put("source.http.certPassword", "wso2carbon");
        InMemoryConfigManager inMemoryConfigManager = new InMemoryConfigManager(masterConfigs, null);
        inMemoryConfigManager.generateConfigReader("source", "http");
        siddhimanager = new SiddhiManager();
        siddhimanager.setConfigManager(inMemoryConfigManager);
    }

    private static void setCarbonHome() {
        java.nio.file.Path carbonHome = Paths.get("");
        carbonHome = Paths.get(carbonHome.toString(), "src", "test");
        System.setProperty(CARBON_HOME, carbonHome.toString());
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
    public Response parseSiddhiApp(SiddhiParserRequest request) throws NotFoundException {
        try {
            List<DeployableSiddhiApp> deployableSiddhiApps = new ArrayList<>();
            List<String> userGivenApps = populateAppWithEnvs(request.getPropertyMap(), request.getSiddhiApps());
            for (String app : userGivenApps) {
                List<SourceDeploymentConfig> sourceDeploymentConfigs = getSourceDeploymentConfigs(app);
                SiddhiTopologyCreator siddhiTopologyCreator = new SiddhiTopologyCreatorImpl();
                SiddhiTopology topology = siddhiTopologyCreator.createTopology(app);
                if (request.getMessagingSystem() != null) {
                    SiddhiAppCreator appCreator = new NatsSiddhiAppCreator();
                    List<DeployableSiddhiQueryGroup> queryGroupList = appCreator.createApps(topology,
                            request.getMessagingSystem());
                    boolean isAppStateful = ((SiddhiTopologyCreatorImpl) siddhiTopologyCreator).isAppStateful();
                    for (DeployableSiddhiQueryGroup deployableSiddhiQueryGroup : queryGroupList) {
                        if (deployableSiddhiQueryGroup.isReceiverQueryGroup()) {
                            for (SiddhiQuery siddhiQuery : deployableSiddhiQueryGroup.getSiddhiQueries()) {
                                deployableSiddhiApps.add(new DeployableSiddhiApp(siddhiQuery.getApp(),
                                        sourceDeploymentConfigs));
                            }
                        } else {
                            for (SiddhiQuery siddhiQuery : deployableSiddhiQueryGroup.getSiddhiQueries()) {
                                DeployableSiddhiApp deployableSiddhiApp = new DeployableSiddhiApp(siddhiQuery.getApp(),
                                        isAppStateful);
                                if (deployableSiddhiQueryGroup.isUserGivenSource()) {
                                    deployableSiddhiApp.setSourceDeploymentConfigs(sourceDeploymentConfigs);
                                }
                                deployableSiddhiApps.add(deployableSiddhiApp);
                            }
                        }
                    }
                } else {
                    DeployableSiddhiApp deployableSiddhiApp = new DeployableSiddhiApp(app);
                    if (((SiddhiTopologyCreatorImpl) siddhiTopologyCreator).isUsergiveSource()) {
                        deployableSiddhiApp.setSourceDeploymentConfigs(sourceDeploymentConfigs);
                    }
                    deployableSiddhiApps.add(deployableSiddhiApp);
                }
            }
            String jsonString = new Gson().toJson(deployableSiddhiApps);
            return Response.ok(jsonString, MediaType.APPLICATION_JSON)
                    .build();
        } catch (Exception e) {
            log.error("Exception caught while parsing the app. ", e);
            return Response.serverError().entity("Exception caught while parsing the app. " + e.getMessage()).build();
        }
    }

    private List<String> populateAppWithEnvs(Map<String, String> envMap, List<String> siddhiApps) {
        List<String> populatedApps = new ArrayList<>();
        if (siddhiApps != null) {
            for (String siddhiApp : siddhiApps) {
                if (siddhiApp.contains("$")) {
                    if (envMap != null) {
                        String envPattern = "\\$\\{(\\w+)\\}";
                        Pattern expr = Pattern.compile(envPattern);
                        Matcher matcher = expr.matcher(siddhiApp);
                        while (matcher.find()) {
                            for (int i = 1; i <= matcher.groupCount(); i++) {
                                String envValue = envMap.getOrDefault(matcher.group(i).toUpperCase(), "");
                                envValue = envValue.replace("\\", "\\\\");
                                Pattern subexpr = Pattern.compile("\\$\\{" + matcher.group(i) + "\\}");
                                siddhiApp = subexpr.matcher(siddhiApp).replaceAll(envValue);
                            }
                        }
                    }
                }
                populatedApps.add(siddhiApp);
            }
        }
        return populatedApps;
    }

    private List<SourceDeploymentConfig> getSourceDeploymentConfigs(String siddhiApp) {
        List<SourceDeploymentConfig> sourceDeploymentConfigs = new ArrayList<>();
        SiddhiAppRuntime siddhiAppRuntime = siddhimanager.createSiddhiAppRuntime(siddhiApp);
        Collection<List<Source>> sources = siddhiAppRuntime.getSources();
        for (List<Source> sourceList : sources) {
            for (Source source : sourceList) {
                SourceDeploymentConfig response = new SourceDeploymentConfig();
                ServiceDeploymentInfo serviceDeploymentInfo = source.getServiceDeploymentInfo();
                if (serviceDeploymentInfo != null) {
                    response.setPort(serviceDeploymentInfo.getPort());
                    response.setServiceProtocol(serviceDeploymentInfo.getServiceProtocol().name());
                    response.setSecured(serviceDeploymentInfo.isSecured());
                    response.setPulling(serviceDeploymentInfo.isPulling());
                    response.setDeploymentProperties(serviceDeploymentInfo.getDeploymentProperties());
                    sourceDeploymentConfigs.add(response);
                }
            }
        }
        return sourceDeploymentConfigs;
    }
}
