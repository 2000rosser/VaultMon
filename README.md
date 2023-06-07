# Custom Kubernetes Operator for Secrets Management Service
# About
This project aims to create a custom operator designed to monitor and manage Vault Custom Resource Definitions (CRDs) within a Kubernetes environment. The operator performs functions like tracking changes to Vault CRDs, monitoring the health status of the replica set, logging the destruction of Vault CRDs, and detecting anomalies. By providing comprehensive Vault monitoring, this operator ensures efficient implementation and effective operation.
# Details
The main functions of the custom Vault operator include:

- Tracking all Vault CRDs with a unique identifier and logging these identifiers.
- Capturing and persisting details of each Vault CRD, such as namespace, labels, annotations, IPs, replicas, endpoints, pod status, volumes, and ingress.
- Logging changes made to the Vault CRDs, like an increase in replica count, changes in memory or CPU, changes in image or version, and changes in ingress.
- Monitoring the health of the replica set, with the capability to detect when health is deteriorating.
- Logging the destruction of Vault CRDs.
- Providing a method for querying the operator data

## Description
The VaultMon Operator is a custom Kubernetes operator created with a focus on providing a comprehensive solution for monitoring and managing Vault Custom Resource Definitions (CRDs).

The operator is designed to keep a log of all Vault CRDs, capturing details like namespace, labels, annotations, IPs, replicas, endpoints, pod status, volumes, and ingress. This data is logged and can be queried, providing you with easy access to granular information about your Vault CRDs.

In addition to logging details, the VaultMon Operator actively monitors changes made to your Vault CRDs. Whether there's an increase in the replica count, changes in memory, CPU usage, image or version, or ingress, the operator is equipped to track these modifications and keep you informed.

The VaultMon Operator is also programmed to continuously monitor the health of the replica set. It alerts you if there are any issues such as a deterioration in CPU and memory usage or changes in Pod statuses. The operator also logs the destruction of Vault CRDs, offering an audit trail for your Vault CRDs lifecycle.

The custom operator is built using the OperatorSDK, providing a streamlined build, test, and deployment process. The operator employs a Custom Resource Definition (CRD) to store Vault-related data and a controller for interacting with the Kubernetes cluster and performing logging operations.

One of the key features of VaultMon Operator is the implementation of metrics. The operator uses the Prometheus package to expose these metrics through an endpoint using PromQL syntax. These metrics can then be scraped and visualized using Prometheus and Grafana or any visualization tool that supports a Prometheus endpoint.

In summary, the VaultMon Operator offers a solution for managing Vault CRDs in Kubernetes. Its features include logging, tracking, health monitoring, and data querying capabilities, offering a comprehensive toolset to enhance the efficiency and effectiveness of your Vault monitoring processes.


## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Deploying Vault
I followed the following [tutorial](https://banzaicloud.com/blog/operator-sdk/) to deploy Vault on the Cluster

### Metrics server
The operator requires the metrics server to be running on the cluster in order to retrieve Vault CPU and Memory usage. To deploy the metrics server you can do the following:

```sh
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.6.3/components.yaml
```

In my case, I had to configure the metrics server to work on my minikube cluster. This step will vary depending on what environment you are deploying the metrics server on.
To deploy it on Minikube you can do the following:

1. Retrieve the deployment yaml 
```sh
kubectl get deployment metrics-server -n kube-system -o yaml > metrics-server-deployment.yaml
```



2. Edit the metrics-server-deployment.yaml file and add the following to the container: -args field: 
```sh
command:
- /metrics-server
- --kubelet-insecure-tls
- --kubelet-preferred-address-types=InternalIP
```

3. Apply the new configuration.
```sh
kubectl apply -f metrics-server-deployment.yaml
```

## Prometheus operator and Grafana
In order to have Grafana work with the operator we need to have Prometheus and Grafana installed on the cluster

We can follow these steps to do that.

1. Install the Prometheus community helm chart
```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update 
```
2. Use the values.yaml file found in the /config/prometheus folder in the project to deploy prometheus and grafana. This file has been modified to only include the necessary deployments

```sh
helm install prometheus prometheus-community/kube-prometheus-stack -f values.yaml
```
3. Port forward:

```sh
kubectl port-forward -n default svc/prometheus-grafana 3000:80
```

Now open localhost:3000/login
user = admin
pass = prom-operator

.
### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/vaultmon:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/vaultmon:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
I would like to sincerely thank Prof. Tahar Kechadi, my supervisor, for his valuable guidance and
insights in planning and report writing. His support has been fundamental to this project. I also
want to express my gratitude to Mr. Arun Thundyill Saseendran from VMware. His technical
knowledge and insightful feedback have significantly enhanced the quality of this work. Their
contributions have been invaluable, and I am deeply grateful for their support.


### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

