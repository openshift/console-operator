package e2e

import (
	"context"
	"net/http"
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/clidownloads"
	"github.com/openshift/console-operator/test/e2e/framework"
	"github.com/openshift/library-go/pkg/route/routeapihelpers"
)

func setupDownloadsTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupDownloadsTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

func TestDownloadsEndpoint(t *testing.T) {
	client, _ := setupDownloadsTestCase(t)
	defer cleanupDownloadsTestCase(t, client)

	route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get route: %s", err)
	}

	url, _, err := routeapihelpers.IngressURI(route, route.Spec.Host)
	if err != nil {
		t.Fatal(err)
	}

	ocDownloads := clidownloads.PlatformBasedOCConsoleCLIDownloads(url.String(), api.OCCLIDownloadsCustomResourceName)

	for _, link := range ocDownloads.Spec.Links {
		req := getRequest(t, link.Href)
		client := getInsecureClient()
		resp, err := client.Do(req)

		if err != nil {
			t.Fatalf("http error getting %s at %s: %s", link.Text, link.Href, err)
		}
		if !httpOK(resp) {
			t.Fatalf("http error for %s at %s: %d %s", link.Text, link.Href, resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		resp.Body.Close()
		t.Logf("%s %s\n", link.Text, resp.Status)
	}
}
