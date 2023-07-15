# Plumberctl
## Overview
The "plumberctl" is a CLI for linking ClusterDetector Controller.  
 Read the kubeconfig file in the plumberctl execution terminal and connect to the Kuberentes Cluster where the Controller is deployed to register or delete clusters and display the federate state of the current cluster.The registration status of Kubernetes Cluster is monitored by ClusterDetector Controller and is reflected in Custom Resource in real time.
## Activation
The kubeconfig file of the plumberctl execution terminal is recorded in the "~/.plumber/toml" file upon activation.
```
$ plumberctl activate --path <Path of kubeconfig file>　--activate-context <Kubernetes Cluster context in which the plumber controller is deployed>
```
The toml file is saved in the following format.
```
$ cat ~/.plumber/config.toml
cluster = 'primary'
path = '/home/user/.kube/config'
```
## Command Usage
### add
Merge the information of the target Kubernetes Cluster to the kubeconfig information in the config Secret of the kubeconfig Namespace of the Kubernetes Cluster specified in activate.
```
$ plumberctl add --target-context <Additional target Kubernetes Cluster context>
```
### remove
Delete the Kubernetes Cluster information from the kubeconfig information in the config Secret of the kubeconfig Namespace of the Kubernetes Cluster specified in activate.
```
$ plumberctl remove --context <Removal target Kubernetes Cluster context>
```
### view
View the current registered Kubernetes Cluster
```
$ plumberctl view
```
The output is as follows.
```
+------------+---------------+-------------------+
|  CONTEXT   |    CLUSER     |       USER        |
+------------+---------------+-------------------+
| primary    | kubernetes    | kubernetes-admin  |
| secondary1 | v1262-cluster | kubernetes-admin2 |
| secondary2 | v1252-cluster | kubernetes-admin3 |
+------------+---------------+-------------------+
```
