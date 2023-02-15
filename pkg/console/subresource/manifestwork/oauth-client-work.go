package manifestwork

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	oauthclientsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

func DefaultManagedClusterOAuthClientManifestWork(operatorConfig *operatorv1.Console, namespace string, secret string, redirects []string) *workv1.ManifestWork {
	manifestWork := ManagedClusterOAuthClientManifestWorkStub(namespace)
	oauthClient := oauthclientsub.DefaultManagedClusterOauthClient(secret, redirects)
	SetServiceAccountExecutor(manifestWork, api.OpenShiftConsoleOperatorExecutor, api.OpenShiftConsoleOperatorNamespace)
	AppendManifest(manifestWork, oauthClient)
	util.AddOwnerRef(manifestWork, util.OwnerRefFrom(operatorConfig))
	return manifestWork
}

func ManagedClusterOAuthClientManifestWorkStub(namespace string) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.ManagedClusterOauthClientManifestWork,
			Namespace: namespace,
			Labels:    util.LabelsForManagedClusterResources(""),
		},
	}
}
