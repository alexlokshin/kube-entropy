package main

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func monitorServices(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.MonitoringSettings.ServiceMonitoring.Selector)
	for true {
		services, err := clientset.CoreV1().Services("").List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of services. Skipping for now. %v\n", err)
		}
		for i := 0; i < len(services.Items); i++ {
			service := services.Items[i]
			if service.Spec.Type == v1.ServiceTypeExternalName {
				// This is just a proxy, it has no pods, nothing to test
				continue
			}

			for _, element := range service.Spec.Ports {
				uri := service.Name + "." + service.Namespace + ".svc:" + element.TargetPort.String()
				if !inCluster {
					// When testing out of cluster, only NodePort and ingress routes can be tested, LoadBalancer and ingresses are not supported yet
					if service.Spec.Type == v1.ServiceTypeNodePort {
						if element.NodePort > 0 {
							uri = ec.MonitoringSettings.ServiceMonitoring.NodePortHost + ":" + strconv.Itoa(int(element.NodePort))
						} else {
							uri = ""
						}
					}
				}

				if len(uri) > 0 {
					go func() {
						log.Printf("Connecting to %s\n", uri)
						conn, err := net.Dial(strings.ToLower(string(element.Protocol)), uri)
						if err != nil {
							log.Printf("could not connect to service %s: %v\n", uri, err)
						}
						defer conn.Close()
					}()
				}
			}
		}
		time.Sleep(ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)
	}
}
