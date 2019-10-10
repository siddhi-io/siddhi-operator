Siddhi Process Specifications Guide
====================================================

## Introduction

In this documentation, we are going to understand the specifications used in the `SiddhiProcess` [custom resource definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

## Deploy Siddhi Apps

The primary objective of the Siddhi operator is to deploy Siddhi apps in a user given Kubernetes cluster according to fulfill user requirements. To deploy Siddhi apps we can use the `apps` spec in the `SiddhiProcess` YAML. 

The users can specify Siddhi apps in two different ways:

1. Siddhi query directly as a string

    ```yaml
	spec:
	  apps:
	    - script: |
	        @App:name("PowerSurgeDetection")
	        @App:description("App consumes events from HTTP as a JSON message of { 'deviceType': 'dryer', 'power': 6000 } format and inserts the events into DevicePowerStream, and alerts the user if the power level is greater than or equal to 600W by printing a message in the log.")
	        
	        @source(
	          type='http',
	          receiver.url='${RECEIVER_URL}',
	          basic.auth.enabled='${BASIC_AUTH_ENABLED}',
	          @map(type='json')
	        )
	        define stream DevicePowerStream(deviceType string, power int);

	        @sink(type='log', prefix='LOGGER')  
	        define stream PowerSurgeAlertStream(deviceType string, power int); 

	        @info(name='surge-detector')  
	        from DevicePowerStream[deviceType == 'dryer' and power >= 600] 
	        select deviceType, power  
	        insert into PowerSurgeAlertStream;
    ```

1. Add Siddhi file to a config map. Then give the config name to the `SiddhiProcess` YAML.

    ```yaml
    spec:
	  apps:
	    - configMap: power-surge-cm1
		- configMap: power-surge-cm2
    ```

## Change Siddhi Runtime Configurations

Internally Siddhi operator has been using Siddhi runner deployments to deploy every Siddhi app. Siddhi runner uses `deployment.yaml` file to contain all the configurations of the Siddhi runner. If you want to change the default Siddhi runner configs you have to use the `runner` spec. For example, the following config will enable file system persistence in the Siddhi runner deployment.

```yaml
  runner: |
    state.persistence:
      enabled: true
      intervalInMin: 1
      revisionsToKeep: 2
      persistenceStore: io.siddhi.distribution.core.persistence.FileSystemPersistenceStore
      config:
        location: siddhi-app-persistence
```

## Change Siddhi Runtime Docker Image and Environment Variables

To change the image and add relevant environmental variables to the Siddhi runtime you can use `container` spec as below.

```yaml
  container: 
    env: 
      - 
        name: RECEIVER_URL
        value: "http://0.0.0.0:8080/checkPower"
      - 
        name: BASIC_AUTH_ENABLED
        value: "false"
    image: "siddhiio/siddhi-runner-ubuntu:latest"
```

You can use the following methods to generate a custom Siddhi runner Docker image.

1. Using the Siddhi tooling Docker and Kubernetes export feature.
1. Using the Docker files in [docker-siddhi](https://github.com/siddhi-io/docker-siddhi) repository.

Here you have to change the default Siddhi runner image whenever you add custom libraries or change the default configs of the Siddhi runtime.

Further, if you need to change the Siddhi runner Docker image for all `SiddhiProcess` deployments you can use `siddhi-operator-config` config map.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: siddhi-operator-config
data:
  siddhiHome: /home/siddhi_user/siddhi-runner/
  siddhiProfile: runner
  siddhiImage: <YOUR-CUSTOM-IMAGE>
  autoIngressCreation: "true"
```

## Use Private Docker Image 

To use a private Docker image for a particular `SiddhiProcess`, you have to create a Kubernetes secret that contained credentials to your private registry as described in [this Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/). Then specify the secret name in the `imagePullSecret` spec.

```yaml
spec:
  imagePullSecret: <SECRET>
```

Further, if you need to pull images from a private registry for all `SiddhiProcess` deployments you can use the `siddhi-operator-config` config map.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: siddhi-operator-config
data:
  siddhiHome: /home/siddhi_user/siddhi-runner/
  siddhiProfile: runner
  siddhiImage: <YOUR-CUSTOM-IMAGE>
  siddhiImageSecret: <SECRET>
  autoIngressCreation: "true"
```

## Setup a Messaging System

You can use the following spec to setup a NATS messaging system cluster for the Siddhi app deployments. Note that, setting up the messaging system indicate that you are going to deploy the Siddhi apps in distributed mode. After that indication, Siddhi operator will create partial Siddhi apps using the given Siddhi app and those apps deployed separately. These partial apps will be connected using the messaging system. There are two cases of specifying the messaging system.

1. If you specify only the type of the messaging system like below, the Siddhi operator will automatically create a NATS cluster and a NATS streaming cluster. Then connect the partial Siddhi apps using the automatically created NATS clusters. The Siddhi operator will automatically create a NATS cluster called `siddhi-nats` and a streaming cluster called `siddhi-stan`.

	```yaml
  	messagingSystem:
      type: nats
	```

1. If you create a NATS messaging system by yourself and then input those configurations in the SiddhiProcess, the partial Siddhi apps will connect to your messaging system. In this example, I have created a NATS cluster called `nats-siddhi` and a NATS streaming cluster called `stan-siddhi`.

	```yaml
	messagingSystem:
	  type: nats
	  config: 
		  bootstrapServers: 
		  - "nats://nats-siddhi:4222"
		  streamingClusterId: stan-siddhi
	```

## Enable Periodic State Persistence

To persists the state of the Siddhi app, you can create a persistence volume first and then specify the PVC configurations in the SiddhiProcess YAML file. This will periodically persist the state of the Siddhi app only if the Siddhi app is a stateful one.

```yaml
  persistentVolumeClaim: 
    accessModes: 
      - ReadWriteOnce
    resources: 
      requests: 
        storage: 1Gi
    storageClassName: standard
    volumeMode: Filesystem
```

If you specify in the default distributed mode of Siddhi deployment, the PVC will bind to the process app because it is the stateful app. The passthrough app is just a stateless app. Note that in default distributed mode, Siddhi operator will be divided into two apps passthrough app and the process app. The passthrough app listens to all the sources and the process app will execute all the queries and output the results.

If you install the Siddhi process in non-distributed mode then the Siddhi operator will create a single pod and mount the PVC to it.

## Disable Automatic Ingress Creation

Since ingress creation will apply to the overall Siddhi operator at once, the ingress creation has to change using operator config YAML. By default, the Siddhi operator will automatically create ingress. To disable that you have to set `autoIngressCreation` configuration to `false`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: siddhi-operator-config
data:
  siddhiHome: /home/siddhi_user/siddhi-runner/
  siddhiProfile: runner
  siddhiImage: <YOUR-CUSTOM-IMAGE>
  siddhiImageSecret: <SECRET>
  autoIngressCreation: "false"
```
