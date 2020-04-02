package e2e

import (
	"context"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupExtensionsTest(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupExtensionsTest(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

func TestCreateCLIDownloadLink(t *testing.T) {
	client, _ := setupExtensionsTest(t)
	defer cleanupExtensionsTest(t, client)

	download := &consolev1.ConsoleCLIDownload{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-cli-download",
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleCLIDownloadSpec{
			DisplayName: "test",
			Description: "test",
			Links: []consolev1.CLIDownloadLink{{
				Text: "download test",
				Href: "https://example.com",
			}},
		},
	}

	download, err := client.ConsoleCliDownloads.Create(context.TODO(), download, v1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsoleCliDownloads custom resource: %v", err)
	}
	err = client.ConsoleCliDownloads.Delete(context.TODO(), download.Name, v1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete ConsoleCliDownloads custom resource: %v", err)
	}
}

func TestCreateExternalLogLink(t *testing.T) {
	client, _ := setupExtensionsTest(t)
	defer cleanupExtensionsTest(t, client)

	externalLogLink := &consolev1.ConsoleExternalLogLink{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-external-log-link",
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleExternalLogLinkSpec{
			Text: "external log link text",
			// this is the template provided in the api docs
			HrefTemplate:    "https://example.com/logs?resourceName=${resourceName}&containerName=${containerName}&resourceNamespace=${resourceNamespace}&podLabels=${podLabels}",
			NamespaceFilter: "^openshift-",
		},
	}

	externalLogLink, err := client.ConsoleExternalLogLink.Create(context.TODO(), externalLogLink, v1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsoleExternalLogLink custom resource: %v", err)
	}
	err = client.ConsoleExternalLogLink.Delete(context.TODO(), externalLogLink.Name, v1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete ConsoleExternalLogLink custom resource: %v", err)
	}

}

func TestCreateLink(t *testing.T) {
	client, _ := setupExtensionsTest(t)
	defer cleanupExtensionsTest(t, client)

	consoleLink := &consolev1.ConsoleLink{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-link",
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Location: "HelpMenu",
			Link: consolev1.Link{
				Text: "test link",
				Href: "https://example.com",
			},
		},
	}
	consoleLink, err := client.ConsoleLink.Create(context.TODO(), consoleLink, v1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsoleLink custom resource: %v", err)
	}
	err = client.ConsoleLink.Delete(context.TODO(), consoleLink.Name, v1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete ConsoleLink custom resource: %v", err)
	}
}

func TestCreateNotification(t *testing.T) {
	client, _ := setupExtensionsTest(t)
	defer cleanupExtensionsTest(t, client)

	notification := &consolev1.ConsoleNotification{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-notification",
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            "test notification",
			Location:        "BannerTop",
			Color:           "#FFFFFF",
			BackgroundColor: "#990000",
		},
	}
	notification, err := client.ConsoleNotification.Create(context.TODO(), notification, v1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsoleNotification custom resource: %v", err)
	}
	err = client.ConsoleNotification.Delete(context.TODO(), notification.Name, v1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete ConsoleNotification custom resource: %v", err)
	}
}

func TestCreateYAMLSample(t *testing.T) {
	client, _ := setupExtensionsTest(t)
	defer cleanupExtensionsTest(t, client)

	yamlSample := &consolev1.ConsoleYAMLSample{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-yaml-sample",
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleYAMLSampleSpec{
			TargetResource: v1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			Title:       "test deployment yaml sample",
			Description: "test deployment yaml sample",
			YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-yaml
  labels:
    app: web`,
			Snippet: false,
		},
	}
	yamlSample, err := client.ConsoleYAMLSample.Create(context.TODO(), yamlSample, v1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsoleYAMLSample custom resource: %v", err)
	}
	err = client.ConsoleYAMLSample.Delete(context.TODO(), yamlSample.Name, v1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete ConsoleYAMLSample custom resource: %v", err)
	}

}
