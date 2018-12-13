package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/extensions/v1beta1"
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

func isMatchingResponse(endpoint EndpointState, resp *http.Response) (result bool, err error) {
	if resp == nil {
		return false, errors.New("No http response available")
	}
	result = true
	if endpoint.Code != resp.StatusCode {
		return false, fmt.Errorf("Status code doesn't match. Expected: %d, Actual: %d", endpoint.Code, resp.StatusCode)
	}

	for headerName, headerValue := range endpoint.Headers {
		if strings.Compare(headerValue, resp.Header.Get(headerName)) != 0 {
			return false, fmt.Errorf("Header values don't match. Expected: %s, Actual: %s", headerValue, resp.Header.Get(headerName))
		}
	}

	return result, nil
}

func getIngressHost(dc discoveryConfig, ingress v1beta1.Ingress, rule v1beta1.IngressRule) (host string) {
	host = dc.Ingress.Protocol + "://" + dc.Ingress.DefaultHost + ":" + dc.Ingress.Port
	if len(strings.TrimSpace(rule.Host)) > 0 {
		protocol := "http"
		port := 80

		if len(ingress.Spec.TLS) > 0 {

		}
		for _, cert := range ingress.Spec.TLS {
			for _, tlsHost := range cert.Hosts {
				if strings.ToLower(tlsHost) == strings.ToLower(rule.Host) {
					protocol = "https"
					port = 443
					break
				}
			}
			if protocol == "https" {
				break
			}
		}

		host = protocol + "://" + strings.TrimSpace(rule.Host) + ":" + strconv.Itoa(port)
	}
	return host
}

func validateIngresses(testPlan ApplicationState) (result bool) {
	result = true
	ingresses := testPlan.Monitoring.Ingresses.Items
	endpoints := []EndpointState{}
	for _, ingress := range ingresses {
		for _, endpoint := range ingress.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
	}

	channel := make(chan bool, len(endpoints))

	for _, endpoint := range endpoints {
		go func(ep EndpointState, channel chan bool) {
			resp, err := http.Get(ep.URL)
			if err != nil {
				// Timeout, DNS doesn't resolve, wrong protocol etc
				log.Printf("Cannot do http GET against %s.\n", ep.URL)
				channel <- false
			} else {
				if match, err := isMatchingResponse(ep, resp); !match {
					log.Printf("Unexpected response when calling %s: %v.\n", ep.URL, err)
					channel <- false
				} else {
					channel <- true
				}
			}
			defer resp.Body.Close()

		}(endpoint, channel)
	}

	for i := 0; i < len(endpoints); i++ {
		if !<-channel {
			result = false
			break
		}
	}
	return result
}

func monitorIngresses(testPlan ApplicationState) {
	for true {
		log.Printf("Checking...")

		validateIngresses(testPlan)

		time.Sleep(testPlan.Monitoring.Interval)
	}
}
