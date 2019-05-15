# Getting Started

Siddhi Operator allows you to run stream processing logic directly on a Kubernetes cluster.
To use it, you need to be connected to a cloud environment or to a local cluster created for development purposes.

## Configure Kubernetes Cluster
### Local Deployment
If you need help on how to create a local development environment based on *Minikube*,
   -  Refer: [Minikube Installation Guide](https://github.com/kubernetes/minikube#installation).

### Google Kubernetes Engine Cluster

Make sure you apply configuration settings for your GKE cluster before installing Siddhi Operator.
   -  Refer: [Configuring a Google Kubernetes Engine Cluster](docs/gke-setup.md)
   
## Enable the NGINX Ingress controller
The Siddhi Operator resource uses the NGINX Ingress Controller to expose the deployments to the external traffic.

In order to enable the NGINX Ingress controller in the desired cloud or on-premise environment,
please refer the official documentation, [NGINX Ingress Controller Installation Guide](https://kubernetes.github.io/ingress-nginx/deploy/).

Supported Version: nginx 0.22.0 

## Install Siddhi Operator in Kubernetes cluster

1. Clone Siddhi Operator Git repository.  
   `git clone https://github.com/siddhi-io/siddhi-operator.git`


2. Execute the following commands to setup the Siddhi Operator in the kubernetes cluster.
   ```
    kubectl create -f ./deploy/crds/siddhi_v1alpha1_siddhiprocess_crd.yaml
    kubectl create -f ./deploy/service_account.yaml
    kubectl create -f ./deploy/role.yaml
    kubectl create -f ./deploy/role_binding.yaml
    kubectl create -f ./deploy/operator.yaml
   ```
    
## Testing a sample

1. Execute the below command to create a sample siddhi deployment.  
`kubectl create -f ./deploy/crds/monitor-app.yaml`

   Siddhi Operator would create a Siddhi-Runner deployment with the Siddhi app deployed through the example-siddhi-app CRD, a service, and an ingress to expose the http endpoint which is in the Siddhi sample.
   
   ```
   $ kubectl get SiddhiProcesses
   NAME          AGE
   monitor-app   2m

   $ kubectl get deployment
   NAME              DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
   monitor-app       1         1         1            1           1m
   siddhi-operator   1         1         1            1           1m
   siddhi-parser     1         1         1            1           1m

   $ kubectl get service
   NAME              TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)          AGE
   kubernetes        ClusterIP      10.96.0.1        <none>        443/TCP          10d
   monitor-app       ClusterIP      10.101.242.132   <none>        8280/TCP         1m
   siddhi-operator   ClusterIP      10.111.138.250   <none>        8383/TCP         1m
   siddhi-parser     LoadBalancer   10.102.172.142   <pending>     9090:31830/TCP   1m

   $ kubectl get ingress
   NAME      HOSTS     ADDRESS     PORTS     AGE
   siddhi    siddhi    10.0.2.15   80, 443   1m
   ```

:information_source: Note:  The Siddhi Operator automatically creates an ingress and exposes the internal endpoints available in the 
Siddhi App by default.
In order to disable the automatic ingress creation, you can set **AUTO_INGRESS_CREATION** environment variable to false/null in
 the `./deploy/operator.yaml`

2. Obtain the external IP (EXTERNAL-IP) of the Ingress resources by listing down the Kubernetes Ingresses.
 
   `kubectl get ing`

3. Add the above host (`siddhi`) as an entry in /etc/hosts file.

4. Use following CURL command to publish an event to the sample Siddhi app that's deployed.
   ```
   curl -X POST \
   http://siddhi/monitor-app/8280/example \
      -H 'Content-Type: application/json' \
      -d '{
         "type": "monitored",
         "deviceID": "001",
         "power": 39
         }`
      
   ```  
5. View the logs of the Siddhi Runner pod and observe the entry being printed by the Siddhi sample app accepting event through the `http` endpoint.
   
   ```
   $ kubectl get pods

   NAME                               READY     STATUS    RESTARTS   AGE
   monitor-app-7f8584875f-krz6t       1/1       Running   0          2m
   siddhi-operator-8589c4fc69-6xbtx   1/1       Running   0          2m
   siddhi-parser-64d4cd86ff-pfq2s     1/1       Running   0          2m

   $ kubectl logs monitor-app-7f8584875f-krz6t
   
   JAVA_HOME environment variable is set to /opt/java/openjdk
   CARBON_HOME environment variable is set to /home/siddhi_user/siddhi-runner-0.1.0
   RUNTIME_HOME environment variable is set to /home/siddhi_user/siddhi-runner-0.1.0/wso2/runner
   Picked up JAVA_TOOL_OPTIONS: -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap
   [2019-04-20 3:58:57,734]  INFO {org.wso2.carbon.launcher.extensions.OSGiLibBundleDeployerUtils updateOSGiLib} - Successfully updated the OSGi bundle information of Carbon Runtime: runner  
   osgi> [2019-04-20 03:59:00,208]  INFO {org.wso2.carbon.config.reader.ConfigFileReader} - Default deployment configuration updated with provided custom configuration file monitor-app-deployment.yaml
   [2019-04-20 03:59:01,551]  INFO {org.wso2.msf4j.internal.websocket.WebSocketServerSC} - All required capabilities are available of WebSocket service component is available.
   [2019-04-20 03:59:01,584]  INFO {org.wso2.carbon.metrics.core.config.model.JmxReporterConfig} - Creating JMX reporter for Metrics with domain 'org.wso2.carbon.metrics'
   [2019-04-20 03:59:01,609]  INFO {org.wso2.msf4j.analytics.metrics.MetricsComponent} - Metrics Component is activated
   [2019-04-20 03:59:01,614]  INFO {org.wso2.carbon.databridge.agent.internal.DataAgentDS} - Successfully deployed Agent Server 
   [2019-04-20 03:59:02,219]  INFO {io.siddhi.distribution.core.internal.ServiceComponent} - Periodic state persistence started with an interval of 5 using io.siddhi.distribution.core.persistence.FileSystemPersistenceStore
   [2019-04-20 03:59:02,229]  INFO {io.siddhi.distribution.event.simulator.core.service.CSVFileDeployer} - CSV file deployer initiated.
   [2019-04-20 03:59:02,233]  INFO {io.siddhi.distribution.event.simulator.core.service.SimulationConfigDeployer} - Simulation config deployer initiated.
   [2019-04-20 03:59:02,279]  INFO {org.wso2.carbon.databridge.receiver.binary.internal.BinaryDataReceiverServiceComponent} - org.wso2.carbon.databridge.receiver.binary.internal.Service Component is activated
   [2019-04-20 03:59:02,312]  INFO {org.wso2.carbon.databridge.receiver.binary.internal.BinaryDataReceiver} - Started Binary SSL Transport on port : 9712
   [2019-04-20 03:59:02,321]  INFO {org.wso2.carbon.databridge.receiver.binary.internal.BinaryDataReceiver} - Started Binary TCP Transport on port : 9612
   [2019-04-20 03:59:02,322]  INFO {org.wso2.carbon.databridge.receiver.thrift.internal.ThriftDataReceiverDS} - Service Component is activated
   [2019-04-20 03:59:02,344]  INFO {org.wso2.carbon.databridge.receiver.thrift.ThriftDataReceiver} - Thrift Server started at 0.0.0.0
   [2019-04-20 03:59:02,356]  INFO {org.wso2.carbon.databridge.receiver.thrift.ThriftDataReceiver} - Thrift SSL port : 7711
   [2019-04-20 03:59:02,363]  INFO {org.wso2.carbon.databridge.receiver.thrift.ThriftDataReceiver} - Thrift port : 7611
   [2019-04-20 03:59:02,449]  INFO {org.wso2.msf4j.internal.MicroservicesServerSC} - All microservices are available
   [2019-04-20 03:59:02,516]  INFO {org.wso2.transport.http.netty.contractimpl.listener.ServerConnectorBootstrap$HttpServerConnector} - HTTP(S) Interface starting on host 0.0.0.0 and port 9090
   [2019-04-20 03:59:02,520]  INFO {org.wso2.transport.http.netty.contractimpl.listener.ServerConnectorBootstrap$HttpServerConnector} - HTTP(S) Interface starting on host 0.0.0.0 and port 9443
   [2019-04-20 03:59:03,068]  INFO {io.siddhi.distribution.core.internal.StreamProcessorService} - Periodic State persistence enabled. Restoring last persisted state of MonitorApp
   [2019-04-20 03:59:03,075]  INFO {org.wso2.transport.http.netty.contractimpl.listener.ServerConnectorBootstrap$HttpServerConnector} - HTTP(S) Interface starting on host 0.0.0.0 and port 8280
   [2019-04-20 03:59:03,077]  INFO {org.wso2.extension.siddhi.io.http.source.HttpConnectorPortBindingListener} - HTTP source 0.0.0.0:8280 has been started
   [2019-04-20 03:59:03,084]  INFO {io.siddhi.distribution.core.internal.StreamProcessorService} - Siddhi App MonitorApp deployed successfully
   [2019-04-20 03:59:03,093]  INFO {org.wso2.carbon.kernel.internal.CarbonStartupHandler} - Siddhi Runner Distribution started in 5.941 sec
   [2019-04-20 04:04:02,216]  INFO {org.wso2.extension.siddhi.io.http.source.HttpSourceListener} - Event input has paused for http://0.0.0.0:8280/example
   [2019-04-20 04:04:02,235]  INFO {org.wso2.extension.siddhi.io.http.source.HttpSourceListener} - Event input has resume for http://0.0.0.0:8280/example
   [2019-04-20 04:05:29,741]  INFO {io.siddhi.core.stream.output.sink.LogSink} - LOGGER : Event{timestamp=1555733129736, data=[monitored, 001, 39], isExpired=false}
   ```

