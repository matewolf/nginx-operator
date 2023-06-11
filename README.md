# NginxOperator

NginxOperator is a Kubernetes operator that is created for deploying and maintaining nginx web server (or nginx web server based) containers.

## Functional desription

As every Kubernetes operator, NginxOperator is also based on a Kubernetes Custom Resource, which has the Kind **NginxOperator**. With the help of **Spec** part of this custom resource the user can modify the behaviour of the operator. The following fields are defined in the Custom Resource Definition of the NginxOperator Kind:
 > * **hostname**: It defines on which hostname the webserver will be reachable from internet.
 > * **image**: It defines which (Nginx web server based) container image should be deployed in the cluster.
 > * **port**: It defines on which port the webserver is waiting for the requests.
 > * **replicas**: It defines how many instance of the web server will be deployed to the cluster.
 > * **issuer**: Operator has the capabilities for setting the TLS encryption for the deployed web server. For this reason it needs an **Issuer** or a **ClusterIssuer** type resource inside the Kubernetes cluster. The expected format of the input is:<br> *\<namespace of the resource>/\<name of the resource>*

NginxOperator is capable of:
> * Deploying Nginx web server based containers which can be accessible on a given hostname and  can be opened using HTTPS secure connection outside from the cluster.
> * Set the TLS encryption for the web server.
> * Handling modifications in the NginxOpetor CR.
> * Repairing the sytem in case of any unexpected event.
> * Deleting the web server in case of deletion of CR.

The operator is continuously trying to harmonize the cluster state with the CR state. It happens in a Reconcile loop. The status of the last try of reconcile is reported in the **ReconcileFailed** condition inside the NginxOperator type Custom Resource. 

## Installation
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.<br>
>_**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows)._

### Prerequisites
#### 1. Ingress controller
Ingress contoller has to be installed on your cluster. [NGINX Ingress controller](https://docs.nginx.com/nginx-ingress-controller/) could be a good choice. In case of KIND you can install it with the following command:
```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```
>_**Note**: Your node has to have the `ingress-ready=true` label._

#### 2. Cert-Manager
Cert-Manager is also needed for running operator. You can install it on any cluster with the following command:
```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
```

#### 3. Install an issuer
An **Issuer** or a **ClusterIssuer** type resource needs to be installed on the cluster. A basic ClusterIssuer can be applied with the following code:
```shell
kubectl apply -f assets/manifests/selfsigned-issuer.yaml
```
or
```
kubectl apply -f assets/manifests/letsencrypt-issuer.yaml
```

### Running on the cluster
#### 1. Deploy the controller to the cluster and all additional resources:

```sh
make deploy
```

#### 2. Install Instance of Custom Resource:
```sh
kubectl apply -f config/samples/operator_v1alpha1_nginxoperator.yaml
```

### Tested on the following versions:
* **Kubernetes**: 1.27
* **Ingress controller**: 1.8.0
* **Cert-manager**: 1.12.0
## Uninstall
To delete the CRDs from the cluster:
```sh
make uninstall
```

UnDeploy the controller from the cluster:
```sh
make undeploy
```

## Run tests
The project contains integration tests. To run integration tests you need a cluster that has the same prerequisites as detailed in the Installation section. Ater that you can run tests with the following command:
```shell
make test
```
>_**Note**: Make sure that run `make undeploy` before running tests._

## License

MIT

