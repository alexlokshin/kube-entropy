![Travis CI Build Status](https://travis-ci.org/alexlokshin/kube-entropy.svg?branch=master "Travis CI Build Status")

# kube-entropy

A little chaos engineering application for kubernetes resilience testing.

## Prerequisites

- Configured kubernetes cluster with an ingress controller deployed
- Ingress controller nodeports mapped to 443 and 80.
- Configured `~/.kube/config`
- Installed kubectl (`brew install kubectl` or https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- Successful execution of `kubectl get nodes`

## Setup

Mofidy `./config/discovery.yaml` to fit your needs. There are two major sections, `nodes` and `ingresses`. s

- `nodes` section allows you to specify whether you want to periodically drain nodes, how often, and which nodes. These settings are under `enabled`, `interval` and `fileds`+`labels` (selectors). Interval can be specified as `10s` or `1h`. `enabled` is a `true` or `false`. `labels` contains a list of filters based on labels, `fields` has a list of filters based on fields. Some examples can be found here: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ . It is a pretty powerful tool.

- `ingresses` section allows you to specify the ingress discovery process. You can specify `fields` and `labels` selectors, `enabled` and `interval` settings like above, but there are three ingress specific settings. `protocol` allows you to specify a default protocol for non-host specific ingresses -- it is either `http` or `https`. Those same ingresses need a default port and a host. In case an ingress route contains a host, we will use that instead. If an ingress has a reference in `tls` pointing to such a host, we will assume it is https on port 443, otherwise, http on port 80.

## Discovery

Run the discovery by executing `./kube-entropy -mode discovery`. It will create a test plan file. We capture a bunch of settings, including full ingress uris, http response codes and key http headers.

## Stress

In this mode, applications are being stressed out based on the test plan, while we continuosly monitor ingress states. If http status changes, or a set of http headers changes (excluding some basic ones, like `Content-Length` or `Set-Cookie`). This indicates an application error or a default backend. Looking at the application logs allows you to determine the source of instability. You might as well can have external monitors enabled. Run this function by executing `./kube-entropy -mode chaos`

```yaml
---
nodes:
  enabled: true
  fields:
    - spec.unschedulable!=true
  labels:
  interval: 5m
ingresses:
  protocol: https
  port: 443
  defaultHost: www.avsatum.com
  selector:
    enabled: true
    interval: 2s
    fields:
      - metadata.namespace=default
      - metadata.namespace!=kube-system
      - metadata.namespace!=docker
    labels:
  successHttpCodes:
    - 2xx
    - 30x
    - 403
```

It is designed to randomly stress two separate events: pod restarts and node drains. Two types of monitoring are supported: service monitoring and ingress monitoring. Each type of monitoring and stress action is independently controlled by labels, selectors, and timing interval.

## In-cluster vs Out of Cluster

## Service monitoring

Designed primarily to keep internal communications in check. If a monitored from within the cluster, service endpoints are invoked directly (only TCP checking is used). If monitoring from the outside of the cluster, node ports are checked against some `nodePortHost`, which is most likely a load balancer. NodePort as well as the service port information is obtained from service definitions. If you use a complex port mapping outside of kubernetes, try deploying kube-entropy into your cluster.

## Ingress monitoring

This type of monitoring is useful to determine if the application responds to ingress requests. As with all kubernetes ingresses, these are reverse proxy routes through the ingress controller (usually nginx), into service and pod IPs. When a pod gets deleted, its IP will be removed from the ingress controller configuration. If the ingress controller doesn't referesh its configuration, an ingress call can be potentially routed to a stale pod IP, which is what we're trying to avoid. Ingress monitoring is HTTP-based, a list of acceptable HTTP codes can be specified in the kube-entropy config file:

```yaml
successHttpCodes:
  - 2xx
  - 3xx
  - 401
```

## Roadmap

- DNS disruption
- Network connectivity disruption
