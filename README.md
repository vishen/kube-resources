# Kubernetes Resource
`kube-resources` shows an entire view of your Kubernetes cluster current
resource usage, including:

* Pod / Container usage
* Pod / Container resource requests
* Pod / Container resource limits
* Node usage
* Node allocatable resources
* Nodes total pod resource requests
* Nodes total pod resource limits

This gives you a high-level overview of what resource requests and limits
have been set for each pod and how what the containers current usage is (
note: this is the containers current cpu and memory usage, and not a usage
overtime).

This will also for a node how much `allocatable` space is left for pod resource
requests and the total resource requests and limits set for all pods for that node.

To determine the resource requests still available you can subtract node
`allocatable` - `resource requests` and that will give you an indication of the pod
requests still available to be set.

By default this will use your current Kubernetes configuration to determine which
cluster to use.

## Installing

    $ go get -u github.com/vishen/kube-resources

## Running
```
$ kube-resources
+-------------+-----------------------------------+---------------------------+------------------+--------------------+--------------------+
|  NAMESPACE  |                POD                |         CONTAINER         |      USAGE       |      REQUESTS      |       LIMITS       |
+-------------+-----------------------------------+---------------------------+------------------+--------------------+--------------------+
| default     | myapp-123412341234-11111          | myapp                     | cpu=10 mem=6Mi   | cpu=50m mem=68Mi   | cpu=100 mem=120Mi  |
| kube-system | event-exporter-...444444444-11111 | event-exporter            | cpu=20 mem=20Mi  | cpu=0 mem=0Mi      | cpu=0 mem=0Mi      |
| kube-system | event-exporter-...444444444-11111 | prometheus-to-sd-exporter | cpu=30 mem=4Mi   | cpu=0 mem=0Mi      | cpu=0 mem=0Mi      |
+-------------+-----------------------------------+---------------------------+------------------+--------------------+--------------------+

+-----------------------------------+--------------------+---------------------+---------------------+---------------------+
|               NODE                |       USAGE        |     ALLOCATABLE     |  RESOURCE REQUESTS  |   RESOURCE LIMITS   |
+-----------------------------------+--------------------+---------------------+---------------------+---------------------+
| gke-k8scluster-...2-11111111-1235 | cpu=60m mem=30Mi   | cpu=900m mem=2000Mi | cpu=50m mem=68Mi    | cpu=100m mem=120Mi  |
+-----------------------------------+--------------------+---------------------+---------------------+---------------------+
```

## TODO
```
- add cli + options for:
    - use specific kubernetes configuration and / or context
    - add selectors for which metrics to show;
        - namespace
        - pod
        - pod selector
        - container
    - determine what headers / columns to show (namespace, pod, nodename, etc)
        - keep current as reasonable defaut, but allow other headers to be used
    - show complete pod / node name
    - only showing pod or node resources
    - sorting based on header(s)
    - sortind based on numeric userage, requests and limits
- add watch command (similar to htop) that will watch and aggregate metrics as they become available
- what kubernetes permissions are required to run this?
```
