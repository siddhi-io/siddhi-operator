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

Supported Version: nginx 0.22.0+

## Install Siddhi Operator in Kubernetes cluster

1. Clone Siddhi Operator Git repository.  
   `git clone https://github.com/siddhi-io/siddhi-operator.git`


2. Execute the following commands to setup the Siddhi Operator in the kubernetes cluster.
   ```sh
    kubectl create -f ./deploy/siddhi_v1alpha2_siddhiprocess_crd.yaml
    kubectl create -f ./deploy/service_account.yaml
    kubectl create -f ./deploy/role.yaml
    kubectl create -f ./deploy/role_binding.yaml
    kubectl create -f ./deploy/operator.yaml
   ```
    
## Testing a sample

1. Execute the below command to create a sample siddhi deployment.  
`kubectl create -f ./deploy/examples/example-stateless-log-app.yaml`

   Siddhi Operator would create a Siddhi-Runner deployment with the Siddhi app deployed through the example-siddhi-app CRD, a service, and an ingress to expose the http endpoint which is in the Siddhi sample.
   
   ```sh
   $ kubectl get SiddhiProcesses
   NAME              STATUS    READY     AGE
   power-surge-app   Running   1/1       2m

   $ kubectl get deployment
   NAME                READY     UP-TO-DATE   AVAILABLE   AGE
   power-surge-app-0   1/1       1            1           2m
   siddhi-operator     1/1       1            1           2m

   $ kubectl get service
   NAME                TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
   kubernetes          ClusterIP   10.96.0.1       <none>        443/TCP    2d
   power-surge-app-0   ClusterIP   10.96.44.182    <none>        8080/TCP   2m
   siddhi-operator     ClusterIP   10.98.78.238    <none>        8383/TCP   2m

   $ kubectl get ingress
   NAME      HOSTS     ADDRESS     PORTS     AGE
   siddhi    siddhi    10.0.2.15   80        2m
   ```

:information_source: Note:  The Siddhi Operator automatically creates an ingress and exposes the internal HTTP/HTTPS endpoints available in the Siddhi App by default.
In order to disable the automatic ingress creation, you have to change the `autoIngressCreation` value in the Siddhi `siddhi-operator-config` config map to `false` or `null`.

2. Obtain the external IP (EXTERNAL-IP) of the Ingress resources by listing down the Kubernetes Ingresses.
 
   `kubectl get ing`

3. Add the above host (`siddhi`) as an entry in /etc/hosts file.

4. Use following CURL command to publish an event to the sample Siddhi app that's deployed.

   ```sh
   curl -X POST \
   http://siddhi/power-surge-app-0/8080/checkPower \
   -H 'Accept: */*' \
   -H 'Content-Type: application/json' \
   -H 'Host: siddhi' \
   -d '{
      "deviceType": "dryer",
      "power": 60000
   }'  
   ```  

5. View the logs of the Siddhi Runner pod and observe the entry being printed by the Siddhi sample app accepting event through the `http` endpoint.
   
   ```sh
   $ kubectl get pods

   NAME                                       READY     STATUS    RESTARTS   AGE
   power-surge-app-0-646c4f9dd5-rxzkq         1/1       Running   0          4m
   siddhi-operator-6698d8f69d-6rfb6           1/1       Running   0          4m

   $ kubectl logs power-surge-app-0-646c4f9dd5-rxzkq

   ...
   [2019-07-12 07:12:48,925]  INFO {org.wso2.transport.http.netty.contractimpl.listener.ServerConnectorBootstrap$HttpServerConnector} - HTTP(S) Interface starting on host 0.0.0.0 and port 9443
   [2019-07-12 07:12:48,927]  INFO {org.wso2.transport.http.netty.contractimpl.listener.ServerConnectorBootstrap$HttpServerConnector} - HTTP(S) Interface starting on host 0.0.0.0 and port 9090
   [2019-07-12 07:12:48,941]  INFO {org.wso2.carbon.kernel.internal.CarbonStartupHandler} - Siddhi Runner Distribution started in 6.853 sec
   [2019-07-12 07:17:22,219]  INFO {io.siddhi.core.stream.output.sink.LogSink} - LOGGER : Event{timestamp=1562915842182, data=[dryer, 60000], isExpired=false}
   ```

Please refer the [Siddhi documentation](https://siddhi.io/en/v5.0/docs/siddhi-as-a-kubernetes-microservice/) for more details about the Siddhi application deployment in Kubernetes.

## Build from Source

### Build the Operator

Clone the operator source repository by executing the below commands.

```sh
$ mkdir $GOPATH/src/github.com/siddhi-io
$ cd $GOPATH/src/github.com/siddhi-io
$ git clone https://github.com/siddhi-io/siddhi-operator.git
```

Build the operator by executing the below command. Replace `DOCKER_REGISTRY_URL` with your private/public docker repository URL where you'll be hosting the Siddhi Operator image.

```sh
$ operator-sdk build <DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>
```

Push the operator as follow.

```sh
$ docker push <DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>
```

Change image name of the `operator.yaml` file.

```sh
$ sed -i 's|docker.io/siddhiio/siddhi-operator:*|<DOCKER_REGISTRY_URL>/<USER_NAME>/siddhi-operator:<TAG>|g' deploy/operator.yaml
```

Now you can install the operator as describe in [previous installation](https://github.com/siddhi-io/siddhi-operator#install-siddhi-operator-in-kubernetes-cluster) section.

### Test the Operator

#### Unit Tests

Execute the below command to start the unit tests.

```sh
$ go test ./pkg/controller/siddhiprocess/<PACKAGE_NAME>
```

For example, run the unit tests for package `artifact`.

```sh
$ go test ./pkg/controller/siddhiprocess/artifact
```

#### E2E Tests

If you have manually made any changes to the Operator, you can verify its functionality with the E2E tests.
Execute the below commands to set up the needed infrastructure for the test-cases.

1. It is recommended to create a separate namespace to test the operator. To do that use the following command.

   ```sh
   $ kubectl create namespace operator-test
   ```

1. After that, you need to install the NATS operator and NATS streaming operator in the `operator-test` namespace. To do that please refer [this documentation](https://github.com/nats-io/nats-streaming-operator/blob/master/README.md).

1. Then you have to set up the siddhi-operator in `operator-test` namespace using following commands.

   ``` sh
   $ kubectl create -f ./deploy/siddhi_v1alpha2_siddhiprocess_crd.yaml --namespace operator-test
   $ kubectl create -f ./deploy/service_account.yaml --namespace operator-test
   $ kubectl create -f ./deploy/role.yaml --namespace operator-test
   $ kubectl create -f ./deploy/role_binding.yaml --namespace operator-test
   $ kubectl create -f ./deploy/operator.yaml --namespace operator-test
   ```

1. Finally, test the operator using following command.

   ```sh
   $ operator-sdk test local ./test/e2e --namespace operator-test --no-setup
   ```

For more details about operator sdk tests, refer [this](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md).
