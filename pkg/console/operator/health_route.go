package operator

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

const (
	routerCABundleKey = "ca-bundle.crt"
)

func (co *consoleOperator) CheckRouteHealth(opConfig *operatorv1.Console, rt *routev1.Route) {
	status.HandleDegraded(func() (conf *operatorv1.Console, prefix string, reason string, err error) {
		prefix = "RouteHealth"

		caPool, err := co.getCA()
		if err != nil {
			return opConfig, prefix, "FailedLoadCA", fmt.Errorf("failed to read CA to check route health: %v", err)
		}
		client := clientWithCA(caPool)

		// TODO: deal with an environment with a MitM proxy?
		url := "https://" + rt.Spec.Host + "/health"
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return opConfig, prefix, "FailedRequest", fmt.Errorf("failed to build request to route (%s): %v", url, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return opConfig, prefix, "FailedGet", fmt.Errorf("failed to GET route (%s): %v", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return opConfig, prefix, "StatusError", fmt.Errorf("route not yet available, %s returns '%s'", url, resp.Status)

		}
		return opConfig, prefix, "", nil
	}())

	status.HandleAvailable(opConfig, "Route", "FailedAdmittedIngress", func() error {
		if !routesub.IsAdmitted(rt) {
			return errors.New("console route is not admitted")
		}
		return nil
	}())
}

func (co *consoleOperator) getCA() (*x509.CertPool, error) {
	// TODO: should I update to this? start with the SystemCertPool?
	//rootCAs, _ := x509.SystemCertPool()
	//if rootCAs == nil {
	//	rootCAs = x509.NewCertPool()
	//}
	caCertPool := x509.NewCertPool()

	routerCA, rcaErr := co.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(api.RouterCAConfigMapName, metav1.GetOptions{})

	if rcaErr != nil && apierrors.IsNotFound(rcaErr) {
		//  using CA ca-bundle.crt from configmap router-ca from openshift-config-managed
		klog.V(4).Infof("using CA [%s] from configmap %s from %s ", routerCABundleKey, api.RouterCAConfigMapName, api.OpenShiftConsoleNamespace)
		var textCABundle string
		textCABundle = routerCA.Data[routerCABundleKey]
		caCertPool.AppendCertsFromPEM([]byte(textCABundle))
		return caCertPool, nil
	}
	// this error is unexpected
	if rcaErr != nil {
		klog.Infof("failed to GET configmap %s in %s (synced from %s)", api.RouterCAConfigMapName, api.OpenShiftConsoleNamespace, api.OpenShiftConfigManagedNamespace)
	}
	// fallback to our local ca in from our serviceaccount
	// if we hit this path, are we likely to get self signed certs errors?
	serviceAccountCAbytes, err := ioutil.ReadFile(api.OAuthEndpointCAFilePath)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failure to read service account ca file: %v\n", err))
	}
	klog.V(4).Infof("using CA on disk from %s", api.OAuthEndpointCAFilePath)
	caCertPool.AppendCertsFromPEM(serviceAccountCAbytes)
	return caCertPool, nil
}

func clientWithCA(caPool *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		// TODO: do I need http.DefaultTransport.(*http.Transport)?
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caPool,
			},
		},
	}
}
