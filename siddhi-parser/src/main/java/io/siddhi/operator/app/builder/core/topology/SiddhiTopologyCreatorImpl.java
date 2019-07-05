/*
 * Copyright (c) 2017, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
 *
 * WSO2 Inc. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package io.siddhi.operator.app.builder.core.topology;

import io.siddhi.core.SiddhiAppRuntime;
import io.siddhi.core.SiddhiManager;
import io.siddhi.operator.app.builder.core.SiddhiTopologyCreator;
import io.siddhi.operator.app.builder.core.util.EventHolder;
import io.siddhi.operator.app.builder.core.util.SiddhiTopologyCreatorConstants;
import io.siddhi.operator.app.builder.core.util.TransportStrategy;
import io.siddhi.query.api.SiddhiApp;
import io.siddhi.query.api.annotation.Annotation;
import io.siddhi.query.api.annotation.Element;
import io.siddhi.query.api.definition.AbstractDefinition;
import io.siddhi.query.api.definition.AggregationDefinition;
import io.siddhi.query.api.definition.StreamDefinition;
import io.siddhi.query.api.exception.SiddhiAppValidationException;
import io.siddhi.query.api.execution.ExecutionElement;
import io.siddhi.query.api.execution.partition.Partition;
import io.siddhi.query.api.execution.partition.PartitionType;
import io.siddhi.query.api.execution.partition.ValuePartitionType;
import io.siddhi.query.api.execution.query.Query;
import io.siddhi.query.api.execution.query.input.handler.StreamFunction;
import io.siddhi.query.api.execution.query.input.handler.StreamHandler;
import io.siddhi.query.api.execution.query.input.handler.Window;
import io.siddhi.query.api.execution.query.input.stream.InputStream;
import io.siddhi.query.api.execution.query.input.stream.JoinInputStream;
import io.siddhi.query.api.execution.query.input.stream.SingleInputStream;
import io.siddhi.query.api.execution.query.input.stream.StateInputStream;
import io.siddhi.query.api.expression.Variable;
import io.siddhi.query.api.util.AnnotationHelper;
import io.siddhi.query.api.util.ExceptionUtil;
import io.siddhi.query.compiler.SiddhiCompiler;
import org.apache.commons.lang3.text.StrSubstitutor;
import org.apache.log4j.Logger;

import java.util.ArrayList;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Map.Entry;
import java.util.Random;
import java.util.Set;
import java.util.UUID;

/**
 * Consumes a Siddhi App and produce a {@link SiddhiTopology} based on distributed annotations.
 */

public class SiddhiTopologyCreatorImpl implements SiddhiTopologyCreator {

    private static final Logger log = Logger.getLogger(SiddhiTopologyCreatorImpl.class);
    private SiddhiTopologyDataHolder siddhiTopologyDataHolder;
    private SiddhiApp siddhiApp;
    private SiddhiAppRuntime siddhiAppRuntime;
    private String userDefinedSiddhiApp;
    boolean transportChannelCreationEnabled = true;

    @Override
    public SiddhiTopology createTopology(String userDefinedSiddhiApp) {
        //parallelism is set to 1, since distributed deployment is not supported at the moment
        int parallelism = 1;
        this.userDefinedSiddhiApp = userDefinedSiddhiApp;
        this.siddhiApp = SiddhiCompiler.parse(userDefinedSiddhiApp);
        this.siddhiAppRuntime = (new SiddhiManager()).createSiddhiAppRuntime(userDefinedSiddhiApp);
        SiddhiQueryGroup siddhiQueryGroup;
        String execGroupName;
        String siddhiAppName = getSiddhiAppName();
        this.siddhiTopologyDataHolder = new SiddhiTopologyDataHolder(siddhiAppName, userDefinedSiddhiApp);
        String defaultExecGroupName = siddhiAppName + "-" + UUID.randomUUID();
        for (ExecutionElement executionElement : siddhiApp.getExecutionElementList()) {
            //execGroupName is set to default, since all elements go under a single group
            execGroupName = defaultExecGroupName;
            siddhiQueryGroup = createSiddhiQueryGroup(execGroupName, parallelism);
            addExecutionElement(executionElement, siddhiQueryGroup, execGroupName);
        }
        //prior to assigning publishing strategies checking if a user given source stream is used in multiple execGroups
        checkUserGivenSourceDistribution();
        assignPublishingStrategyOutputStream();
        cleanInnerGroupStreams(siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values());
        if (log.isDebugEnabled()) {
            log.debug("Topology was created with " + siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values().size
                    () + " parse groups. Following are the partial Siddhi apps.");
            for (SiddhiQueryGroup debugSiddhiQueryGroup : siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values()) {
                log.debug(debugSiddhiQueryGroup.getSiddhiApp());
            }
        }
        return new SiddhiTopology(siddhiTopologyDataHolder.getSiddhiAppName(), new ArrayList<>
                (siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values()), transportChannelCreationEnabled);
    }

    /**
     * Clean input and output streams of the group by removing streams that are only used within that group and
     * validate partitions within groups with parallelism greater than one.
     *
     * @param siddhiQueryGroups Collection of Siddhi Query Groups
     */
    private void cleanInnerGroupStreams(Collection<SiddhiQueryGroup> siddhiQueryGroups) {

        for (SiddhiQueryGroup siddhiQueryGroup : siddhiQueryGroups) {
            for (InputStreamDataHolder inputStreamDataHolder : siddhiQueryGroup.getInputStreams().values()) {
                if (inputStreamDataHolder.isInnerGroupStream()) {
                    for (ExecutionElement element : siddhiApp.getExecutionElementList()) {
                        if (element instanceof Partition && ((Partition) element).getQueryList().get(0).getInputStream()
                                .getAllStreamIds().contains(inputStreamDataHolder.getStreamName()) &&
                                siddhiQueryGroup.getParallelism() > SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL) {
                            throw new SiddhiAppValidationException("Partial Siddhi App " +
                                    siddhiQueryGroup.getName() + " has a "
                                    + "partition which consumes from another "
                                    + "parse inside the same parse group with "
                                    + "parallelism greater than one. "
                                    + "Partitions can only have parallelism "
                                    + "greater than one if and only if they "
                                    + "are consuming from user given stream or"
                                    + " stream from another group. Hence "
                                    + "failing the deployment of Siddhi App.");

                        }
                    }
                }
            }
            siddhiQueryGroup.getInputStreams()
                    .entrySet().removeIf(stringInputStreamDataHolderEntry -> stringInputStreamDataHolderEntry.getValue()
                    .isInnerGroupStream());
            siddhiQueryGroup.getOutputStreams()
                    .entrySet().removeIf(
                    stringOutputStreamDataHolderEntry -> stringOutputStreamDataHolderEntry.getValue()
                            .isInnerGroupStream());
        }
    }

    /**
     * Get SiddhiAppName if given by the user unless a unique SiddhiAppName is returned.
     *
     * @return String SiddhiAppName
     */
    private String getSiddhiAppName() {
        Element element =
                AnnotationHelper.getAnnotationElement(SiddhiTopologyCreatorConstants.SIDDHIAPP_NAME_IDENTIFIER,
                        null, siddhiApp.getAnnotations());
        if (element == null) {
            return SiddhiTopologyCreatorConstants.DEFAULT_SIDDHIAPP_NAME + "-" + UUID.randomUUID(); //defaultName
        } else {
            return element.getValue();
        }
    }

    /**
     * If the corresponding {@link SiddhiQueryGroup} exists that object will be returned unless new object is created.
     */
    private SiddhiQueryGroup createSiddhiQueryGroup(String execGroupName, int parallel) {
        SiddhiQueryGroup siddhiQueryGroup;
        if (!siddhiTopologyDataHolder.getSiddhiQueryGroupMap().containsKey(execGroupName)) {
            siddhiQueryGroup = new SiddhiQueryGroup(execGroupName, parallel);
        } else {
            siddhiQueryGroup = siddhiTopologyDataHolder.getSiddhiQueryGroupMap().get(execGroupName);
            //An executionGroup should have a constant parallel number
            if (siddhiQueryGroup.getParallelism() != parallel) {
                throw new SiddhiAppValidationException(
                        "execGroup = " + execGroupName + " not assigned constant @dist(parallel)");
            }
        }
        return siddhiQueryGroup;
    }

    /**
     * If {@link Query,Partition} string contains {@link SiddhiTopologyCreatorConstants#DISTRIBUTED_IDENTIFIER},
     * meta info related to distributed deployment is removed.
     */
    private String removeMetaInfoQuery(ExecutionElement executionElement, String queryElement) {
        int[] queryContextStartIndex;
        int[] queryContextEndIndex;

        for (Annotation annotation : executionElement.getAnnotations()) {
            if (annotation.getName().toLowerCase()
                    .equals(SiddhiTopologyCreatorConstants.DISTRIBUTED_IDENTIFIER)) {
                queryContextStartIndex = annotation.getQueryContextStartIndex();
                queryContextEndIndex = annotation.getQueryContextEndIndex();
                queryElement = queryElement.replace(
                        ExceptionUtil.getContext(queryContextStartIndex, queryContextEndIndex,
                                siddhiTopologyDataHolder.getUserDefinedSiddhiApp()), "");
                break;
            }
        }
        return queryElement;
    }

    /**
     * Get Map of {@link InputStreamDataHolder} corresponding to a {@link Query}.
     *
     * @return Map of  StreamID and {@link InputStreamDataHolder}
     */
    private Map<String, InputStreamDataHolder> getInputStreamHolderInfo(Query executionElement,
                                                                        SiddhiQueryGroup siddhiQueryGroup,
                                                                        boolean isQuery) {
        int parallel = siddhiQueryGroup.getParallelism();
        String execGroupName = siddhiQueryGroup.getName();
        StreamDataHolder streamDataHolder;
        InputStream inputStream = (executionElement).getInputStream();
        if (inputStream instanceof StateInputStream){
            siddhiTopologyDataHolder.setStatefulApp(true);
        } else if (inputStream instanceof SingleInputStream){
            List<StreamHandler> streamHandlers = ((SingleInputStream)executionElement.getInputStream()).getStreamHandlers();
            for (StreamHandler streamHandler: streamHandlers) {
                if(streamHandler instanceof Window || streamHandler instanceof StreamFunction){
                    siddhiTopologyDataHolder.setStatefulApp(true);
                }
            }
        }
        Map<String, InputStreamDataHolder> inputStreamDataHolderMap = new HashMap<>();
        for (String inputStreamId : inputStream.getUniqueStreamIds()) {
            //not an inner Stream
            if (!inputStreamId.startsWith(SiddhiTopologyCreatorConstants.INNERSTREAM_IDENTIFIER)) {
                streamDataHolder = extractStreamHolderInfo(inputStreamId, parallel, execGroupName);
                TransportStrategy transportStrategy = findStreamSubscriptionStrategy(isQuery, inputStreamId, parallel,
                        execGroupName);

                InputStreamDataHolder inputStreamDataHolder = siddhiQueryGroup.getInputStreams().get(inputStreamId);
                //conflicting strategies for an stream in same in same execGroup
                if (inputStreamDataHolder != null && inputStreamDataHolder
                        .getSubscriptionStrategy().getStrategy() != transportStrategy) {
                    //if parse given before partition --> Partitioned Stream--> RR and FG ->accept
                    if (!(inputStreamDataHolder.getSubscriptionStrategy().getStrategy()
                            .equals(TransportStrategy.ROUND_ROBIN) && transportStrategy
                            .equals(TransportStrategy.FIELD_GROUPING))) {
                        //if parse given before partition --> Unpartitioned Stream --> RR and ALL ->don't
                        throw new SiddhiAppValidationException("Unsupported: " + inputStreamId + " in execGroup "
                                + execGroupName + " having conflicting strategies.");
                    }
                }
                String partitionKey = findPartitionKey(inputStreamId, isQuery);
                inputStreamDataHolder = new InputStreamDataHolder(inputStreamId,
                        streamDataHolder.getStreamDefinition(),
                        streamDataHolder.getEventHolderType(),
                        streamDataHolder.isUserGiven(),
                        new SubscriptionStrategyDataHolder(parallel, transportStrategy, partitionKey));
                inputStreamDataHolderMap.put(inputStreamId, inputStreamDataHolder);

            }
        }
        return inputStreamDataHolderMap;
    }

    /**
     * Get {@link OutputStreamDataHolder} for an OutputStream of a {@link Query}.
     *
     * @return {@link OutputStreamDataHolder}
     */
    private OutputStreamDataHolder getOutputStreamHolderInfo(String outputStreamId, int parallel,
                                                             String execGroupName) {
        if (!outputStreamId.startsWith(SiddhiTopologyCreatorConstants.INNERSTREAM_IDENTIFIER)) {
            StreamDataHolder streamDataHolder = extractStreamHolderInfo(outputStreamId, parallel,
                    execGroupName);
            return new OutputStreamDataHolder(outputStreamId, streamDataHolder.getStreamDefinition(),
                    streamDataHolder.getEventHolderType(), streamDataHolder.isUserGiven());
        } else {
            return null;
        }
    }

    /**
     * If streamID corresponds to a partitioned Stream, partition-key is returned unless null value is returned.
     */
    private String findPartitionKey(String streamID, boolean isQuery) {
        return siddhiTopologyDataHolder.getPartitionKeyMap().get(streamID);
    }

    /**
     * Extract primary information corresponding to {@link InputStreamDataHolder} and {@link OutputStreamDataHolder}.
     * Information is retrieved and assigned to {@link StreamDataHolder} .
     *
     * @return StreamDataHolder
     */
    private StreamDataHolder extractStreamHolderInfo(String streamId, int parallel, String groupName) {
        String streamDefinition;
        int[] queryContextEndIndex;
        int[] queryContextStartIndex;
        StreamDataHolder streamDataHolder = new StreamDataHolder(true);
        Map<String, StreamDefinition> streamDefinitionMap = siddhiApp.getStreamDefinitionMap();
        boolean isUserGivenTransport;
        if (streamDefinitionMap.containsKey(streamId)) {
            StreamDefinition definition = streamDefinitionMap.get(streamId);
            queryContextStartIndex = definition.getQueryContextStartIndex();
            queryContextEndIndex = definition.getQueryContextEndIndex();
            streamDefinition = ExceptionUtil.getContext(queryContextStartIndex, queryContextEndIndex,
                    siddhiTopologyDataHolder.getUserDefinedSiddhiApp());
            isUserGivenTransport = isUserGivenTransport(streamDefinition);
            if (!isUserGivenTransport && !siddhiApp.getTriggerDefinitionMap().containsKey(streamId)) {
                streamDefinition = "${" + streamId + "}" + streamDefinition;
            }
            streamDataHolder =
                    new StreamDataHolder(streamDefinition, EventHolder.STREAM, isUserGivenTransport);

        } else if (siddhiApp.getTableDefinitionMap().containsKey(streamId)) {
            AbstractDefinition tableDefinition = siddhiApp.getTableDefinitionMap().get(streamId);
            streamDataHolder.setEventHolderType(EventHolder.INMEMORYTABLE);

            for (Annotation annotation : tableDefinition.getAnnotations()) {
                if (annotation.getName().toLowerCase().equals(
                        SiddhiTopologyCreatorConstants.PERSISTENCETABLE_IDENTIFIER)) {
                    streamDataHolder.setEventHolderType(EventHolder.TABLE);
                    siddhiTopologyDataHolder.setStatefulApp(true);
                    break;
                }
            }
            //In-Memory tables will not be supported when parallel >1
            if (parallel != SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL &&
                    streamDataHolder.getEventHolderType().equals(EventHolder.INMEMORYTABLE)) {
                throw new SiddhiAppValidationException("Unsupported: "
                        + groupName + " with In-Memory Table  having parallel > 1 ");
            }

            queryContextStartIndex = siddhiApp.getTableDefinitionMap().get(streamId).getQueryContextStartIndex();
            queryContextEndIndex = siddhiApp.getTableDefinitionMap().get(streamId).getQueryContextEndIndex();
            streamDataHolder.setStreamDefinition(ExceptionUtil.getContext(queryContextStartIndex,
                    queryContextEndIndex, siddhiTopologyDataHolder.getUserDefinedSiddhiApp()));

            if (streamDataHolder.getEventHolderType().equals(EventHolder.INMEMORYTABLE) && siddhiTopologyDataHolder
                    .getInMemoryMap().containsKey(streamId)) {
                if (!siddhiTopologyDataHolder.getInMemoryMap().get(streamId).equals(groupName)) {
                    throw new SiddhiAppValidationException("Unsupported:Event Table "
                            + streamId + " In-Memory Table referenced from more than one execGroup: execGroup "
                            + groupName + " && " + siddhiTopologyDataHolder.getInMemoryMap().get(streamId));
                }
            } else {
                siddhiTopologyDataHolder.getInMemoryMap().put(streamId, groupName);
            }

        } else if (siddhiApp.getWindowDefinitionMap().containsKey(streamId)) {
            if (parallel != SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL) {
                throw new SiddhiAppValidationException("Unsupported: "
                        + groupName + " with (Defined) Window " + " having parallel >1");
            }
            siddhiTopologyDataHolder.setStatefulApp(true);
            queryContextStartIndex = siddhiApp.getWindowDefinitionMap().get(streamId).getQueryContextStartIndex();
            queryContextEndIndex = siddhiApp.getWindowDefinitionMap().get(streamId).getQueryContextEndIndex();

            streamDataHolder.setStreamDefinition(ExceptionUtil.getContext(queryContextStartIndex,
                    queryContextEndIndex, siddhiTopologyDataHolder.getUserDefinedSiddhiApp()));
            streamDataHolder.setEventHolderType(EventHolder.WINDOW);

            if (siddhiTopologyDataHolder.getInMemoryMap().containsKey(streamId)) {
                if (!siddhiTopologyDataHolder.getInMemoryMap().get(streamId).equals(groupName)) {
                    throw new SiddhiAppValidationException("Unsupported:(Defined) Window "
                            + streamId + " In-Memory window referenced from more than one execGroup: execGroup "
                            + groupName + " && " + siddhiTopologyDataHolder.getInMemoryMap().get(streamId));
                }
            } else {
                siddhiTopologyDataHolder.getInMemoryMap().put(streamId, groupName);
            }

            //if stream definition is an inferred definition
        } else if (streamDataHolder.getStreamDefinition() == null) {
            if (siddhiAppRuntime.getStreamDefinitionMap().containsKey(streamId)) {
                streamDataHolder = new StreamDataHolder(
                        "${" + streamId + "}"
                                + siddhiAppRuntime.getStreamDefinitionMap().get(streamId).toString(),
                        EventHolder.STREAM, false);
            }
        }
        return streamDataHolder;
    }

    /**
     * Transport strategy corresponding to an InputStream is calculated.
     *
     * @return {@link TransportStrategy}
     */
    private TransportStrategy findStreamSubscriptionStrategy(boolean isQuery, String streamId, int parallel,
                                                             String execGroup) {
        if (parallel > SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL) {
            //partitioned stream residing in partition/ partition + parse of the same execGroup
            if (siddhiTopologyDataHolder.getPartitionGroupMap().containsKey(streamId) &&
                    siddhiTopologyDataHolder.getPartitionGroupMap().get(streamId).contains(execGroup)) {
                return TransportStrategy.FIELD_GROUPING;
            } else if (siddhiApp.getAggregationDefinitionMap().containsKey(streamId)) {
                return TransportStrategy.ROUND_ROBIN;
            } else {
                if (!isQuery) {
                    //inside a partition but not a partitioned stream
                    return TransportStrategy.ALL;
                } else {
                    return TransportStrategy.ROUND_ROBIN;
                }
            }
        } else {
            return TransportStrategy.ALL;
        }
    }

    /**
     * Checks whether a given Stream definition contains Sink or Source configurations.
     */
    private boolean isUserGivenTransport(String streamDefinition) {
        return streamDefinition.toLowerCase().contains(
                SiddhiTopologyCreatorConstants.SOURCE_IDENTIFIER) || streamDefinition.toLowerCase().contains
                (SiddhiTopologyCreatorConstants.SINK_IDENTIFIER);
    }


    /**
     * Checks if the Distributed SiddhiApp contains InputStreams with @Source Configurations which are used by multiple
     * execGroups.Such inputStreams will be added to a separate execGroup as a separate SiddhiApp including a completed
     * {@link SiddhiTopologyCreatorConstants#DEFAULT_PASSTROUGH_QUERY_TEMPLATE} passthrough Queries.
     * <p>
     * Note: The passthrough parse will create an internal Stream which will enrich awaiting excGroups.
     */
    private void checkUserGivenSourceDistribution() {
        int i = 0;
        boolean createPassthrough;          //create passthrough parse for each user given source stream
        boolean passthroughQueriesAvailable = false;           //move passthrough parse to front of SiddhiQueryGroupList

        List<SiddhiQueryGroup> siddhiQueryGroupsList =
                new ArrayList<>(siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values());
        List<SiddhiQueryGroup> passthroughQueries = new ArrayList<>();

        for (SiddhiQueryGroup siddhiQueryGroup1 : siddhiQueryGroupsList) {
            for (Entry<String, InputStreamDataHolder> entry : siddhiQueryGroup1.getInputStreams().entrySet()) {
                String streamId = entry.getKey();
                InputStreamDataHolder inputStreamDataHolder = entry.getValue();
                if (inputStreamDataHolder.getEventHolderType() != null && inputStreamDataHolder
                        .getEventHolderType().equals(EventHolder.STREAM) &&
                        inputStreamDataHolder.isUserGivenSource()) {
                    createPassthrough = true;
                    String runtimeDefinition = removeMetaInfoStream(streamId,
                            inputStreamDataHolder.getStreamDefinition(), SiddhiTopologyCreatorConstants
                                    .SOURCE_IDENTIFIER);

                    for (Annotation annotation : siddhiApp.getStreamDefinitionMap()
                            .get(inputStreamDataHolder.getStreamName()).getAnnotations()) {
                        if(isUsergiveSource()){
                            siddhiQueryGroup1.setUserGivenSource(true);
                            if (annotation.getElement("type").equalsIgnoreCase("nats")) {
                                siddhiQueryGroup1.setMessagingSource(true);
                            }
                        }
                    }
                    if (!siddhiQueryGroup1.isMessagingSource()
                            && siddhiTopologyDataHolder.isStatefulApp()) {
                        passthroughQueriesAvailable = true;
                        passthroughQueries.addAll(generatePassthroughQueryList(
                                streamId, inputStreamDataHolder, runtimeDefinition));
                        inputStreamDataHolder.setStreamDefinition(runtimeDefinition);
                        inputStreamDataHolder.setUserGiven(false);
                        siddhiQueryGroup1.setUserGivenSource(false);
                        InputStreamDataHolder holder = siddhiQueryGroup1.getInputStreams().get(streamId);
                        String consumingStream = "${" + streamId + "} " + removeMetaInfoStream(streamId,
                                holder.getStreamDefinition(), SiddhiTopologyCreatorConstants.SOURCE_IDENTIFIER);
                        holder.setStreamDefinition(consumingStream);
                        holder.setUserGiven(false);
                        createPassthrough = false;
                    }

                    for (SiddhiQueryGroup siddhiQueryGroup2 : siddhiQueryGroupsList.subList(i + 1,
                            siddhiQueryGroupsList.size())) {
                        if (siddhiQueryGroup2.getInputStreams().containsKey(streamId)) {
                            String consumingStream = "";
                            runtimeDefinition = removeMetaInfoStream(streamId,
                                    inputStreamDataHolder.getStreamDefinition(), SiddhiTopologyCreatorConstants
                                            .SOURCE_IDENTIFIER);
                            passthroughQueriesAvailable = true;
                            //A passthrough parse will be created only once for an inputStream satisfying above
                            // requirements.
                            if (createPassthrough) {
                                passthroughQueries.addAll(generatePassthroughQueryList(
                                        streamId, inputStreamDataHolder, runtimeDefinition));
                                inputStreamDataHolder.setStreamDefinition(runtimeDefinition);
                                inputStreamDataHolder.setUserGiven(false);
                                siddhiQueryGroup1.setUserGivenSource(false);
                                createPassthrough = false;
                                InputStreamDataHolder holder1 = siddhiQueryGroup1.getInputStreams().get(streamId);
                                consumingStream = "${" + streamId + "} " + removeMetaInfoStream(streamId,
                                        holder1.getStreamDefinition(),
                                        SiddhiTopologyCreatorConstants.SOURCE_IDENTIFIER);
                                holder1.setStreamDefinition(consumingStream);
                                holder1.setUserGiven(false);

                            }
                            InputStreamDataHolder holder2 = siddhiQueryGroup2.getInputStreams().get(streamId);
                            consumingStream = "${" + streamId + "} " + removeMetaInfoStream(streamId,
                                    holder2.getStreamDefinition(), SiddhiTopologyCreatorConstants.SOURCE_IDENTIFIER);
                            holder2.setStreamDefinition(consumingStream);
                            holder2.setUserGiven(false);
                        }
                    }
                }
            }
            i++;
        }
        //Created SiddhiQueryGroup object will be moved as the first element of the linkedHashMap of already created
        // SiddhiApps only if at least one Passthrough parse is being created.
        if (passthroughQueriesAvailable) {
            addFirst(passthroughQueries);
        }
    }

    private List<SiddhiQueryGroup> generatePassthroughQueryList(String streamId,
                                                                InputStreamDataHolder inputStreamDataHolder,
                                                                String runtimeDefinition) {
        List<SiddhiQueryGroup> passthroughQueryGroupList = new ArrayList<>();
        StreamDefinition definition = siddhiApp.getStreamDefinitionMap().get(streamId);
        for (Annotation annotation : definition.getAnnotations()) {
            if (annotation.getName().equalsIgnoreCase("source")){
                int parallelism = getSourceParallelism(annotation);
                SiddhiQueryGroup passthroughQueryGroup = createPassthroughQueryGroup(inputStreamDataHolder,
                        runtimeDefinition, parallelism);
                passthroughQueryGroup.setReceiverQueryGroup(true);
                passthroughQueryGroupList.add(passthroughQueryGroup);
            }
        }
        return passthroughQueryGroupList;
    }

    /**
     * If the Stream definition string contains {@link SiddhiTopologyCreatorConstants#SINK_IDENTIFIER} or
     * {@link SiddhiTopologyCreatorConstants#SOURCE_IDENTIFIER} ,meta info related to Sink/Source configuration is
     * removed.
     *
     * @return Stream definition String after removing Sink/Source configuration
     */
    private String removeMetaInfoStream(String streamId, String streamDefinition, String identifier) {

        int[] queryContextStartIndex;
        int[] queryContextEndIndex;

        for (Annotation annotation : siddhiApp.getStreamDefinitionMap().get(streamId).getAnnotations()) {
            if (annotation.getName().toLowerCase().equals(identifier.replace("@", ""))) {
                queryContextStartIndex = annotation.getQueryContextStartIndex();
                queryContextEndIndex = annotation.getQueryContextEndIndex();
                streamDefinition = streamDefinition.replace(
                        ExceptionUtil.getContext(queryContextStartIndex, queryContextEndIndex,
                                siddhiTopologyDataHolder.getUserDefinedSiddhiApp()), "");
            }
        }
        return streamDefinition;
    }

    /**
     * PassthroughSiddhiQueryGroups will be moved as the first elements of the existing linkedHashMap of
     * SiddhiQueryGroups.
     *
     * @param passthroughSiddhiQueryGroupList List of Passthrough queries
     */
    private void addFirst(List<SiddhiQueryGroup> passthroughSiddhiQueryGroupList) {

        Map<String, SiddhiQueryGroup> output = new LinkedHashMap();
        for (SiddhiQueryGroup passthroughSiddhiQueryGroup : passthroughSiddhiQueryGroupList) {
            output.put(passthroughSiddhiQueryGroup.getName(), passthroughSiddhiQueryGroup);
        }
        output.putAll(siddhiTopologyDataHolder.getSiddhiQueryGroupMap());
        siddhiTopologyDataHolder.getSiddhiQueryGroupMap().clear();
        siddhiTopologyDataHolder.getSiddhiQueryGroupMap().putAll(output);
    }

    /**
     * Passthrough parse is created using {@link SiddhiTopologyCreatorConstants#DEFAULT_PASSTROUGH_QUERY_TEMPLATE}
     * and required {@link InputStreamDataHolder} and {@link OutputStreamDataHolder} are assigned to the
     * SiddhiQueryGroup.
     */
    private SiddhiQueryGroup createPassthroughQueryGroup(InputStreamDataHolder inputStreamDataHolder,
                                                         String runtimeDefinition, int parallelism) {

        String passthroughExecGroupName = siddhiTopologyDataHolder.getSiddhiAppName() + "-" +
                SiddhiTopologyCreatorConstants.PASSTHROUGH + "-" + new Random().nextInt(99999);
        SiddhiQueryGroup siddhiQueryGroup = new SiddhiQueryGroup(passthroughExecGroupName,
                parallelism);
        String streamId = inputStreamDataHolder.getStreamName();
        Map<String, String> valuesMap = new HashMap();
        String inputStreamID = SiddhiTopologyCreatorConstants.PASSTHROUGH + inputStreamDataHolder.getStreamName();
        valuesMap.put(SiddhiTopologyCreatorConstants.INPUTSTREAMID, inputStreamID);
        valuesMap.put(SiddhiTopologyCreatorConstants.OUTPUTSTREAMID, streamId);
        StrSubstitutor substitutor = new StrSubstitutor(valuesMap);
        String passThroughQuery = substitutor.replace(SiddhiTopologyCreatorConstants.DEFAULT_PASSTROUGH_QUERY_TEMPLATE);
        siddhiQueryGroup.addQuery(passThroughQuery);
        String inputStreamDefinition = inputStreamDataHolder.getStreamDefinition().replace(streamId, inputStreamID);
        String outputStreamDefinition = "${" + streamId + "} " + runtimeDefinition;
        siddhiQueryGroup.getInputStreams()
                .put(inputStreamID, new InputStreamDataHolder(inputStreamID,
                        inputStreamDefinition, EventHolder.STREAM, true,
                        new SubscriptionStrategyDataHolder(
                                SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL,
                                TransportStrategy.ALL, null)));
        siddhiQueryGroup.getOutputStreams().put(streamId, new OutputStreamDataHolder(streamId, outputStreamDefinition,
                EventHolder.STREAM, false));
        return siddhiQueryGroup;
    }

    /**
     * Adds the given execution element to the SiddhiQueryGroup.
     *
     * @param executionElement The execution element to be added to the SiddhiQueryGroup.
     * @param siddhiQueryGroup SiddhiQueryGroup which the execution element to be added.
     * @param queryGroupName   Name of the above SiddhiQueryGroup.
     */
    private void addExecutionElement(ExecutionElement executionElement, SiddhiQueryGroup siddhiQueryGroup,
                                     String queryGroupName) {
        int[] queryContextEndIndex;
        int[] queryContextStartIndex;
        int parallelism = siddhiQueryGroup.getParallelism();
        if (executionElement instanceof Query) {
            //set parse string
            queryContextStartIndex = ((Query) executionElement).getQueryContextStartIndex();
            queryContextEndIndex = ((Query) executionElement).getQueryContextEndIndex();
            siddhiQueryGroup.addQuery(removeMetaInfoQuery(executionElement, ExceptionUtil
                    .getContext(queryContextStartIndex, queryContextEndIndex, userDefinedSiddhiApp)));
            siddhiQueryGroup.addInputStreams(getInputStreamHolderInfo((Query) executionElement,
                    siddhiQueryGroup, true));
            String outputStreamId = ((Query) executionElement).getOutputStream().getId();
            siddhiQueryGroup.addOutputStream(outputStreamId, getOutputStreamHolderInfo(outputStreamId, parallelism,
                    queryGroupName));
        } else if (executionElement instanceof Partition) {
            siddhiTopologyDataHolder.setStatefulApp(true);
            //set Partition string
            queryContextStartIndex = ((Partition) executionElement)
                    .getQueryContextStartIndex();
            queryContextEndIndex = ((Partition) executionElement).getQueryContextEndIndex();
            siddhiQueryGroup.addQuery(removeMetaInfoQuery(executionElement, ExceptionUtil
                    .getContext(queryContextStartIndex, queryContextEndIndex, userDefinedSiddhiApp)));

            //store partition details
            storePartitionInfo((Partition) executionElement, queryGroupName);
            //for a partition iterate over containing queries to identify required inputStreams and OutputStreams
            for (Query query : ((Partition) executionElement).getQueryList()) {
                siddhiQueryGroup.addInputStreams(getInputStreamHolderInfo(query, siddhiQueryGroup, false));
                String outputStreamId = query.getOutputStream().getId();
                siddhiQueryGroup.addOutputStream(outputStreamId, getOutputStreamHolderInfo(outputStreamId,
                        parallelism, queryGroupName));
            }

        }
        siddhiTopologyDataHolder.getSiddhiQueryGroupMap().put(queryGroupName, siddhiQueryGroup);
    }


    /**
     * Publishing strategies for an OutputStream is assigned if the corresponding outputStream is being used as an
     * InputStream in a separate execGroup.
     */
    private void assignPublishingStrategyOutputStream() {
        int i = 0;
        List<SiddhiQueryGroup> siddhiQueryGroupsList =
                new ArrayList<>(siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values());

        for (SiddhiQueryGroup siddhiQueryGroup1 : siddhiQueryGroupsList) {
            for (Entry<String, OutputStreamDataHolder> entry : siddhiQueryGroup1.getOutputStreams().entrySet()) {
                OutputStreamDataHolder outputStreamDataHolder = entry.getValue();
                String streamId = entry.getKey();

                if (outputStreamDataHolder.getEventHolderType().equals(EventHolder.STREAM)) {
                    Map<String, List<SubscriptionStrategyDataHolder>> fieldGroupingSubscriptions = new HashMap<>();
                    boolean isInnerGroupStream = true;
                    for (SiddhiQueryGroup siddhiQueryGroup2 : siddhiQueryGroupsList.subList(i + 1,
                            siddhiQueryGroupsList.size())) {
                        if (siddhiQueryGroup2.getInputStreams().containsKey(streamId)) {
                            isInnerGroupStream = false;
                            InputStreamDataHolder inputStreamDataHolder = siddhiQueryGroup2.getInputStreams()
                                    .get(streamId);

                            //Checks if the Distributed SiddhiApp contains OutputStreams with Sink Configurations
                            //which are used as an inputStream in a different execGroup
                            //If above then a  outputStream Sink configuration is concatenated with placeholder
                            //corresponding to the outputStreamId
                            //InputStream definition will be replaced.
                            if (outputStreamDataHolder.isUserGiven()) {
                                String runtimeStreamDefinition = removeMetaInfoStream(streamId,
                                        inputStreamDataHolder.getStreamDefinition(),
                                        SiddhiTopologyCreatorConstants.SINK_IDENTIFIER);

                                if (!outputStreamDataHolder.isSinkBridgeAdded()) {
                                    String outputStreamDefinition = outputStreamDataHolder.
                                            getStreamDefinition().replace(runtimeStreamDefinition, "\n"
                                            + "${" + streamId + "} ") + runtimeStreamDefinition;
                                    outputStreamDataHolder.setStreamDefinition(outputStreamDefinition);
                                    outputStreamDataHolder.setSinkBridgeAdded(true);
                                }

                                inputStreamDataHolder.setStreamDefinition("${" + streamId + "} "
                                        + runtimeStreamDefinition);
                                inputStreamDataHolder.setUserGiven(false);
                            }

                            SubscriptionStrategyDataHolder subscriptionStrategy = inputStreamDataHolder.
                                    getSubscriptionStrategy();
                            if (subscriptionStrategy.getStrategy().equals(TransportStrategy.FIELD_GROUPING)) {
                                String partitionKey = subscriptionStrategy.getPartitionKey();
                                if (fieldGroupingSubscriptions.containsKey(partitionKey)) {
                                    fieldGroupingSubscriptions.get(partitionKey).add(
                                            subscriptionStrategy);
                                } else {
                                    List<SubscriptionStrategyDataHolder> strategyList = new ArrayList<>();
                                    strategyList.add(subscriptionStrategy);
                                    fieldGroupingSubscriptions.put(partitionKey, strategyList);
                                }

                            } else {
                                outputStreamDataHolder.addPublishingStrategy(
                                        new PublishingStrategyDataHolder(subscriptionStrategy.getStrategy(),
                                                siddhiQueryGroup2.getParallelism()));
                            }

                        }
                    }
                    if (isInnerGroupStream && !outputStreamDataHolder.isUserGiven()) {
                        siddhiQueryGroup1.getOutputStreams().get(streamId).setInnerGroupStream(true);
                        if (siddhiQueryGroup1.getInputStreams().get(streamId) != null) {
                            siddhiQueryGroup1.getInputStreams().get(streamId).setInnerGroupStream(true);
                        }

                    }
                    for (Entry<String, List<SubscriptionStrategyDataHolder>> subscriptionParentEntry :
                            fieldGroupingSubscriptions.entrySet()) {
                        String partitionKey = subscriptionParentEntry.getKey();
                        List<SubscriptionStrategyDataHolder> strategyList = subscriptionParentEntry.getValue();
                        strategyList.sort(new StrategyParallelismComparator().reversed());
                        int parallelism = strategyList.get(0).getOfferedParallelism();
                        for (SubscriptionStrategyDataHolder holder : strategyList) {
                            holder.setOfferedParallelism(parallelism);
                        }
                        outputStreamDataHolder.addPublishingStrategy(
                                new PublishingStrategyDataHolder(
                                        TransportStrategy.FIELD_GROUPING, partitionKey, parallelism));
                    }
                }

            }
            i++;
        }
    }

    /**
     * Details required while processing Partitions are stored.
     */
    private void storePartitionInfo(Partition partition, String execGroupName) {
        List<String> partitionGroupList; //contains all the execGroups containing partitioned streamId
        String partitionKey;

        //assign partitionGroupMap
        for (Entry<String, PartitionType> partitionTypeEntry : partition.getPartitionTypeMap().entrySet()) {

            String streamID = partitionTypeEntry.getKey();
            if (siddhiTopologyDataHolder.getPartitionGroupMap().containsKey(streamID)) {
                partitionGroupList = siddhiTopologyDataHolder.getPartitionGroupMap().get(streamID);
            } else {
                partitionGroupList = new ArrayList<>();
            }

            partitionGroupList.add(execGroupName);
            siddhiTopologyDataHolder.getPartitionGroupMap().put(streamID, partitionGroupList);

            if (partitionTypeEntry.getValue() instanceof ValuePartitionType) {
                partitionKey = ((Variable) ((ValuePartitionType) partitionTypeEntry.getValue()).getExpression())
                        .getAttributeName();

                //More than one partition corresponding to same partition-key of a stream can not reside in the same
                // execGroup.
                if (siddhiTopologyDataHolder.getPartitionKeyGroupMap().get(streamID + partitionKey) != null &&
                        siddhiTopologyDataHolder.getPartitionKeyGroupMap().get(streamID + partitionKey)
                                .equals(execGroupName)) {
                    throw new SiddhiAppValidationException("Unsupported in distributed setup :More than 1 partition "
                            + "residing on the same execGroup " + execGroupName + " for " + streamID + " "
                            + partitionKey);
                }
                siddhiTopologyDataHolder.getPartitionKeyGroupMap().put(streamID + partitionKey, execGroupName);
                siddhiTopologyDataHolder.getPartitionKeyMap().put(streamID, partitionKey);
                updateInputStreamDataHolders(streamID, partitionKey);
            } else {
                //Not yet supported
                throw new SiddhiAppValidationException("Unsupported: "
                        + execGroupName + " Range PartitionType not Supported in Distributed SetUp");
            }
        }
    }

    private void updateInputStreamDataHolders(String streamID, String partitionKey) {
        for (SiddhiQueryGroup siddhiQueryGroup : siddhiTopologyDataHolder.getSiddhiQueryGroupMap().values()) {
            InputStreamDataHolder holder = siddhiQueryGroup.getInputStreams().get(streamID);
            if (holder != null && holder.getSubscriptionStrategy().getStrategy() != TransportStrategy.FIELD_GROUPING) {
                holder.getSubscriptionStrategy().setPartitionKey(partitionKey);
            }
        }
    }

    /**
     * Provide the parallelism count for pass-through parse. In case of multiple user given values,
     * lowest one will be selected. Default parallelism value would be 1.
     *
     * @param annotation Source annotation
     * @return Parallelism count for pass-through parse
     */
    private int getSourceParallelism(Annotation annotation) {
        Set<Integer> parallelismSet = new HashSet<>();
        List<Annotation> distAnnotations = annotation.getAnnotations(
                SiddhiTopologyCreatorConstants.DISTRIBUTED_IDENTIFIER);
        if (distAnnotations.size() > 0) {
            for (Annotation distAnnotation : distAnnotations) {
                if (distAnnotation.getElement(SiddhiTopologyCreatorConstants.PARALLEL_IDENTIFIER) != null) {
                    parallelismSet.add(Integer.valueOf(distAnnotation.getElement(
                            SiddhiTopologyCreatorConstants.PARALLEL_IDENTIFIER)));
                } else {
                    return SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL;
                }
            }
        } else {
            return SiddhiTopologyCreatorConstants.DEFAULT_PARALLEL;
        }
        return parallelismSet.stream().min(Comparator.comparing(Integer::intValue)).get();
    }

    public boolean isAppStateful() {
        return siddhiTopologyDataHolder.isStatefulApp();
    }

    public boolean isUsergiveSource() {
        Map<String, StreamDefinition> stringStreamDefinitionMap = siddhiApp.getStreamDefinitionMap();
        for (StreamDefinition streamDef : stringStreamDefinitionMap.values()
                ) {
            for (Annotation annotation : streamDef.getAnnotations()) {
                if (annotation.getName().equalsIgnoreCase("source")) {
                    return true;
                }
            }
        }
        return false;
    }
}
