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

package parser

// Constants for the parser
const (
	NATSMessagingType string = "nats"
	NATSDefaultURL    string = "nats://siddhi-nats:4222"
	ParserParameter   string = "-Dsiddhi-parser "
	ParserName        string = "parser"
	ParserHTTP        string = "http://"
	ParserExtension   string = "-parser"
	ParserHealth      string = ".svc.cluster.local:9090/health"
	ParserContext     string = ".svc.cluster.local:9090/siddhi-parser/parse"
	ParserPort        int32  = 9090
	ParserReplicas    int32  = 1
	ParserMinWait     int    = 3
	ParserMaxWait     int    = 10
	ParserMaxRetry    int    = 40
)
