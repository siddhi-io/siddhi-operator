# Getting Started

Siddhi Operator allows you to run stream processing logic directly on a Kubernetes cluster.
To use it, you need to be connected to a cloud environment or to a local cluster created for development purposes.

## Prerequisites
### Running the Operator
- Kubernetes v1.10.11+
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) version v1.11.3+

### Building the Operator
- [operator-sdk](https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md)
- [dep](https://golang.github.io/dep/docs/installation.html) version v0.5.0+
- [git](https://git-scm.com/downloads)
- [go](https://golang.org/dl/) version v1.12+
- [docker](https://docs.docker.com/install/) version 17.03+
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster


## Configure Kubernetes Cluster
### Local Deployment

#### Minikube
Refer [Minikube Installation Guide](https://github.com/kubernetes/minikube#installation) to setup a local kubernetes cluster with *Minikube*.

#### Docker for Mac 
Refer [Docker for Mac Installation Guide](https://docs.docker.com/docker-for-mac/install/) setup a local kubernetes cluster with Docker for Mac.

### Google Kubernetes Engine Cluster

Make sure you apply configuration settings for your GKE cluster before installing Siddhi Operator.
   -  Refer: [Configuring a Google Kubernetes Engine Cluster](docs/gke-setup.md)
   
## Enable the NGINX Ingress controller
The Siddhi Operator resource uses the NGINX Ingress Controller to expose the deployments to the external traffic.

In order to enable the NGINX Ingress controller in the desired cloud or on-premise environment,
please refer the official documentation, [NGINX Ingress Controller Installation Guide](https://kubernetes.github.io/ingress-nginx/deploy/).

Supported Version: nginx 0.21.0/0.22.0 

## Install Siddhi Operator in Kubernetes cluster

1. Clone Siddhi Operator Git repository.  
   `git clone https://github.com/siddhi-io/siddhi-operator.git`


2. Execute the following commands to setup the Siddhi Operator in the kubernetes cluster.
   ```
    kubectl create -f ./deploy/crds/siddhi_v1alpha2_siddhiprocess_crd.yaml
    kubectl create -f ./deploy/service_account.yaml
    kubectl create -f ./deploy/role.yaml
    kubectl create -f ./deploy/role_binding.yaml
    kubectl create -f ./deploy/operator.yaml
   ```
    
## Testing a sample

1. Execute the below command to create a sample siddhi deployment.  
`kubectl create -f ./deploy/crds/examples/example-http-default-app.yaml`

   Siddhi Operator would create a Siddhi-Runner deployment with the Siddhi app deployed through the example-siddhi-app CRD, a service, and an ingress to expose the http endpoint which is in the Siddhi sample.
   
   ```
   $ kubectl get SiddhiProcesses
   NAME          STATUS    READY     AGE
   monitor-app   Running   1/1       2m

   $ kubectl get deployment
   NAME              READY     UP-TO-DATE   AVAILABLE   AGE
   monitor-app-0     1/1       1            1           3m13s
   siddhi-operator   1/1       1            1           5m12s
   siddhi-parser     1/1       1            1           5m12s

   $ kubectl get service
   NAME              TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
   kubernetes        ClusterIP   10.96.0.1        <none>        443/TCP    2d13h
   monitor-app-0     ClusterIP   10.107.254.75    <none>        8080/TCP   3m53s
   siddhi-operator   ClusterIP   10.110.56.7      <none>        8383/TCP   5m47s
   siddhi-parser     ClusterIP   10.110.136.148   <none>        9090/TCP   5m52s

   $ kubectl get ingress
   NAME      HOSTS     ADDRESS     PORTS     AGE
   siddhi    siddhi    10.0.2.15   80        6m19s
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
   http://siddhi/monitor-app-0/8080/example \
   -H 'Accept: */*' \
   -H 'Content-Type: application/json' \
   -H 'Host: siddhi' \
   -d '{
      "type": "monitored",
      "deviceID": "001",
      "power":1
   }'
      
   ```  
5. View the logs of the Siddhi Runner pod and observe the entry being printed by the Siddhi sample app accepting event through the `http` endpoint.
   
   ```
   $ kubectl get pods

   NAME                               READY     STATUS    RESTARTS   AGE
   monitor-app-0-685cf4685c-2tb4p     1/1       Running   0          10m
   siddhi-operator-7c8c7f9b99-kkhj4   1/1       Running   0          12m
   siddhi-parser-76959867b6-lv4ck     1/1       Running   0          12m

   $ kubectl logs monitor-app-0-685cf4685c-2tb4p
   
   JAVA_HOME environment variable is set to /opt/java/openjdk
   CARBON_HOME environment variable is set to /home/siddhi_user/siddhi-runner
   RUNTIME_HOME environment variable is set to /home/siddhi_user/siddhi-runner/wso2/runner
   Picked up JAVA_TOOL_OPTIONS: -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap
   [2019-07-07 7:03:01,554]  INFO {org.wso2.carbon.launcher.extensions.OSGiLibBundleDeployerUtils updateOSGiLib} - Successfully updated the OSGi bundle information of Carbon Runtime: runner  
   osgi> [2019-07-07 07:03:05,016]  INFO {org.wso2.carbon.config.reader.ConfigFileReader} - Default deployment configuration updated with provided custom configuration file monitor-app-depyml
   ......
   ......
   [2019-07-07 07:12:14,547]  INFO {io.siddhi.core.stream.output.sink.LogSink} - LOGGER : Event{timestamp=1562483534542, data=[001, 1], isExpired=false}
   ```

## Build from Source

### Build the Operator

Clone the operator source repository by executing the below commands.
```
$ mkdir $GOPATH/src/github.com/siddhi-io
$ cd $GOPATH/src/github.com/siddhi-io
$ git clone https://github.com/siddhi-io/siddhi-operator.git
```

Build the operator by executing the below command. Replace `DOCKER_REGISTRY_URL` with your private/public docker repository URL where you'll be hosting the Siddhi Operator image.
```
$ operator-sdk build <DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>
```

Push the operator as follow.
```
$ docker push <DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>
```
Change image name of the `operator.yaml` file.
```
$ sed -i 's|docker.io/siddhiio/siddhi-operator:*|<DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>|g' deploy/operator.yaml
```
Now you can install the operator as describe in [previous installation](https://github.com/siddhi-io/siddhi-operator#install-siddhi-operator-in-kubernetes-cluster) section.

### Test the Operator

#### Unit Tests

Execute the below command to start the unit tests.
```
$ go test ./pkg/controller/siddhiprocess/
```

#### E2E Tests

If you have manually made any changes to the Operator, you can verify its functionality with the E2E tests.
Execute the below commands to set up the needed infrastructure for the test-cases.

1. It is recommended to create a separate namespace to test the operator. To do that use the following command.
   ```
   $ kubectl create namespace operator-test
   ```
1. After that, you need to install the NATS operator and NATS streaming operator in the `operator-test` namespace. To do that please refer [this documentation](https://github.com/nats-io/nats-streaming-operator/blob/master/README.md).

1. Then you have to set up the siddhi-operator in `operator-test` namespace using following commands.
   ``` 
   $ kubectl create -f ./deploy/crds/siddhi_v1alpha2_siddhiprocess_crd.yaml --namespace operator-test
   $ kubectl create -f ./deploy/service_account.yaml --namespace operator-test
   $ kubectl create -f ./deploy/role.yaml --namespace operator-test
   $ kubectl create -f ./deploy/role_binding.yaml --namespace operator-test
   $ kubectl create -f ./deploy/operator.yaml --namespace operator-test
   ```
1. Finally, test the operator using following command.   
   ```
   $ operator-sdk test local ./test/e2e --namespace operator-test --no-setup
   ```
For more details about operator sdk tests, refer [this](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md).
