package e2e

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	operatorsv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupMetricsEndpointTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	client, _ := framework.StandardSetup(t)
	routeForTest := tempRouteForTesting()
	_, err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Create(routeForTest)
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("error: %s", err)
	}
	return client, nil
}

func cleanUpMetricsEndpointTestCase(t *testing.T, client *framework.ClientSet) {
	routeForTest := tempRouteForTesting()
	err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Delete(routeForTest.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	framework.StandardCleanup(t, client)
}

// This test essentially does the following to verify the `console_url` is in the metrics endpoint:
// 	curl --insecure -H "Authorization: Bearer $(oc whoami --show-token)" $(oc get route metrics --template 'https://{{.spec.host}}/metrics') | grep console_url
// alternatively:
// 	oc exec -it console-operator-<pod-id> /bin/bash
// 	curl --insecure -H "Authorization: Bearer <token>  https://localhost:8443/metrics | grep console_url
func TestMetricsEndpoint(t *testing.T) {
	client, _ := setupMetricsEndpointTestCase(t)
	defer cleanUpMetricsEndpointTestCase(t, client)

	metricsURL := getMetricsURL(t, client)
	fmt.Printf("fetching metrics url.... %v\n", metricsURL)

	fmt.Printf("making request...\n")
	respString := metricsRequest(t, metricsURL)

	toFind := "console_url"
	fmt.Printf("searching for %v\n", toFind)
	found := findLineInResponse(respString, toFind)
	if !found {
		t.Fatalf("did not find %v", toFind)
	}
}

func findLineInResponse(haystack, needle string) (found bool) {
	scanner := bufio.NewScanner(strings.NewReader(haystack))
	for scanner.Scan() {
		found := strings.Contains(scanner.Text(), needle)
		if found {
			fmt.Printf("found %v\n", scanner.Text())
			return true
		}
	}
	return false
}

func metricsRequest(t *testing.T, routeForMetrics string) string {
	bearer := getBearerToken(t)
	req := getRequest(t, routeForMetrics, bearer)
	insecureClient := getInsecureClient()

	resp, err := insecureClient.Do(req)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	return string(bytes)
}

// a request with a route & the bearer token
func getRequest(t *testing.T, metricsURL, bearer string) *http.Request {
	req, err := http.NewRequest("GET", metricsURL, nil)
	if err != nil {

	}
	req.Header.Add("Authorization", bearer)
	return req
}

func getInsecureClient() *http.Client {
	// ignore self signed certs for testing purposes
	insecureTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	insecureClient := &http.Client{
		Transport: insecureTransport,
	}
	return insecureClient
}

// gets us a token from the kubeconfig file
func getBearerToken(t *testing.T) string {
	tokenProvider, err := framework.GetConfig()
	if err != nil {
		t.Fatalf("error, can't get config: %s", err)
	}
	// build the request header with a token from the kubeconfig
	return fmt.Sprintf("Bearer %s", tokenProvider.BearerToken)
}

// creates a temp route, polls for the route, and returns just a formatted url for https://<host>/metrics
func getMetricsURL(t *testing.T, client *framework.ClientSet) string {
	tempRoute := tempRouteForTesting()
	routeForMetrics := ""
	err := wait.Poll(1*time.Second, 30*time.Second, func() (stop bool, err error) {
		tempRoute, err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Get(tempRoute.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("error: %s", err)
		}
		if len(tempRoute.Spec.Host) == 0 {
			return false, err
		}
		routeForMetrics = "https://" + tempRoute.Spec.Host + "/metrics"
		t.Logf("route for metrics: %v", routeForMetrics)
		return true, nil
	})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	t.Logf("route to /metrics: (%v) \n", routeForMetrics)
	return routeForMetrics
}

// In production our metrics endpoint should not have a route, but this makes it
// easier to access the pod http://localhost:8443/metrics endpoint to verify its output
func tempRouteForTesting() *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics",
			Namespace: api.OpenShiftConsoleOperatorNamespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "metrics",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}
