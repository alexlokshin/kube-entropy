# kube-entropy
A little chaos engineering application for kubernetes resilience testing.

It is designed to randomly stress two separate events: pod restarts and node drains. Two types of monitoring are supported: service monitoring and ingress monitoring. Each type of monitoring and stress action is independently controlled by labels, selectors, and timing interval.

## In-cluster vs Out of Cluster

## Service monitoring

Designed primarily to keep internal communications in check. If a monitored from within the cluster, service endpoints are invoked directly (only TCP checking is used). If monitoring from the outside of the cluster, node ports are checked against some `nodePortHost`, which is most likely a load balancer. NodePort as well as the service port information is obtained from service definitions. If you use a complex port mapping  outside of kubernetes, try deploying kube-entropy into your cluster. 

## Ingress monitoring

This type of monitoring is useful to determine if the application responds to ingress requests. As with all kubernetes ingresses, these are reverse proxy routes through the ingress controller (usually nginx), into service and pod IPs. When a pod gets deleted, its IP will be removed from the ingress controller configuration. If the ingress controller doesn't referesh its configuration, an ingress call can be potentially routed to a stale pod IP, which is what we're trying to avoid. Ingress monitoring is HTTP-based, a list of acceptable HTTP codes can be specified in the kube-entropy config file:

```yaml
successHttpCodes:
  - 2xx
  - 3xx
  - 401
```