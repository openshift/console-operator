package e2e

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
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

const (
	consoleURLMetric = "console_url"
)

func setupMetricsEndpointTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	client, _ := framework.StandardSetup(t)
	routeForTest := tempRouteForTesting()
	_, err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Create(context.TODO(), routeForTest, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("error: %s", err)
	}
	return client, nil
}

func cleanUpMetricsEndpointTestCase(t *testing.T, client *framework.ClientSet) {
	routeForTest := tempRouteForTesting()
	err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Delete(context.TODO(), routeForTest.Name, metav1.DeleteOptions{})
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
	// TODO Fix this flaky test case - https://bugzilla.redhat.com/show_bug.cgi?id=2046497
	t.Skip("Skipping TestMetricsEndpoint case. See https://bugzilla.redhat.com/show_bug.cgi?id=2046497")
	client, _ := setupMetricsEndpointTestCase(t)
	defer cleanUpMetricsEndpointTestCase(t, client)

	metricsURL := getMetricsURL(t, client)
	t.Logf("fetching metrics url.... %s\n", metricsURL)

	t.Logf("fetching /metrics data...\n")
	respString := metricsRequest(t, metricsURL)

	t.Logf("searching for %s in metrics data...\n", consoleURLMetric)
	found := findLineInResponse(t, respString, consoleURLMetric)
	if !found {
		t.Fatalf("did not find %s", consoleURLMetric)
	}
}

func findLineInResponse(t *testing.T, haystack, needle string) (found bool) {
	scanner := bufio.NewScanner(strings.NewReader(haystack))
	for scanner.Scan() {
		text := scanner.Text()
		// skip comments
		if strings.HasPrefix(text, "#") {
			continue
		}
		found := strings.Contains(text, needle)
		if found {
			t.Logf("found %s\n", scanner.Text())
			return true
		}
	}
	return false
}

func metricsRequest(t *testing.T, routeForMetrics string) string {
	req := getRequest(t, routeForMetrics)
	httpClient := getClientWithCertAuth(t)

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("http error: %s", err)
	}

	if !httpOK(resp) {
		t.Fatalf("http error: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read error: %s", err)
	}

	return string(bytes)
}

// kubeadmin is an oauth account. it will not work. to run tests instead do:
// oc login -u system:admin
// so that the config read gives correct data back.
// this only works if test are run as a cert based user (example: system:admin)
func getClientWithCertAuth(t *testing.T) *http.Client {
	config, err := framework.GetConfig()
	if err != nil {
		t.Fatalf("error, can't get kube config: %s", err)
	}

	// load from memory, not file
	tlsCert, err := tls.X509KeyPair(config.CertData, config.KeyData)
	// load from file, not memory
	// tlsCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)

	if err != nil {
		t.Fatalf("error, cannot get key pair, are you logged in as system:admin (kube:admin will not work)? %s", err)
	}

	rootCAs, err := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if err != nil {
		// not sure if we should panic/die here
		fmt.Printf("x509 certs err: %v", err)
	}

	// need to add the CAData, the kubeconfig has router certs
	// that are missing from the system trust roots
	rootCAs.AppendCertsFromPEM(config.CAData)

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				RootCAs:      rootCAs,
				// x509 error, certificate is valid for service, not route, so must
				// use an insecure client. This is not really ideal after going through
				// the trouble of wiring up the certs :/
				InsecureSkipVerify: true,
			},
		},
	}
}

// polls for the metrics route host, and returns https://<host>/metrics
func getMetricsURL(t *testing.T, client *framework.ClientSet) string {
	tempRoute := tempRouteForTesting()
	routeForMetrics := ""
	err := wait.Poll(1*time.Second, 30*time.Second, func() (stop bool, err error) {
		tempRoute, err := client.Routes.Routes(api.OpenShiftConsoleOperatorNamespace).Get(context.TODO(), tempRoute.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("error: %s", err)
		}
		if len(tempRoute.Spec.Host) == 0 {
			return false, err
		}
		routeForMetrics = "https://" + tempRoute.Spec.Host + "/metrics"
		t.Logf("route for metrics: %s", routeForMetrics)
		return true, nil
	})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	t.Logf("route to /metrics: (%s)", routeForMetrics)
	return routeForMetrics
}

// in production the operator does not have a route.
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
				// Termination:                   routev1.TLSTerminationReencrypt,
				// NOTE: has to be passthrough for cert auth?
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}
