# kube-entropy
A little chaos engineering application for kubernetes resilience testing.

It is designed to randomly stress two separate events: pod restarts and node drains. Two types of monitoring are supported: service monitoring and ingress monitoring. Each type of monitoring and stress action is independently controlled by labels, selectors, and timing interval.
