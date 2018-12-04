package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// IsSuccessHTTPCode determines if the passed http code matches one of the masks provided
func IsSuccessHTTPCode(validCodes []string, code string) (result bool) {
	for _, validCode := range validCodes {
		if len(code) == len(validCode) && strings.HasPrefix(code, strings.TrimRight(validCode, "x")) {
			return true
		}
	}
	return false
}

func getIngressHost(rule v1beta1.IngressRule) (host string) {
	host = ec.MonitoringSettings.IngressMonitoring.Protocol + "://" + ec.MonitoringSettings.IngressMonitoring.DefaultHost + ":" + ec.MonitoringSettings.IngressMonitoring.Port
	if len(strings.TrimSpace(rule.Host)) > 0 {
		protocol := "https"
		if len(ec.MonitoringSettings.IngressMonitoring.Protocol) > 0 {
			protocol = ec.MonitoringSettings.IngressMonitoring.Protocol
		}
		port := "443"
		if len(ec.MonitoringSettings.IngressMonitoring.Port) > 0 {
			port = ec.MonitoringSettings.IngressMonitoring.Port
		}
		host = protocol + "://" + strings.TrimSpace(rule.Host) + ":" + port
	}
	return host
}

func monitorIngresses(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.MonitoringSettings.IngressMonitoring.Selector)
	for true {
		ingresses, err := clientset.Extensions().Ingresses("").List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of ingresses. Skipping for now. %v\n", err)
		}
		for _, ingress := range ingresses.Items {
			for _, rule := range ingress.Spec.Rules {
				host := getIngressHost(rule)
				for _, path := range rule.HTTP.Paths {
					uri := host + path.Path
					go func() {
						resp, err := http.Get(uri)
						if err != nil {
							// Timeout, DNS doesn't resolve, wrong protocol etc
							log.Printf("Cannot do http GET against %s.\n", uri)
						} else {
							if !IsSuccessHTTPCode(ec.MonitoringSettings.IngressMonitoring.SuccessHTTPCodes, strconv.Itoa(resp.StatusCode)) {
								log.Printf("Unexpected http code %d when calling %s.\n", resp.StatusCode, uri)
							}
						}
						defer resp.Body.Close()
					}()
				}
			}
		}
		time.Sleep(ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)
	}
}
