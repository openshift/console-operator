package manifestwork

import (
	"errors"

	oauthclientsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	workv1 "open-cluster-management.io/api/work/v1"
)

// Appends an object to manifests.
func AppendManifest(manifestWork *workv1.ManifestWork, object runtime.Object) {
	if &manifestWork.Spec == nil {
		manifestWork.Spec = workv1.ManifestWorkSpec{}
	}

	if &manifestWork.Spec.Workload == nil {
		manifestWork.Spec.Workload = workv1.ManifestsTemplate{}
	}

	if &manifestWork.Spec.Workload.Manifests == nil {
		manifestWork.Spec.Workload.Manifests = []workv1.Manifest{}
	}

	manifestWork.Spec.Workload.Manifests = append(
		manifestWork.Spec.Workload.Manifests,
		workv1.Manifest{RawExtension: runtime.RawExtension{Object: object}},
	)
}

// Sets the ServiceAccount executor
func SetServiceAccountExecutor(manifestWork *workv1.ManifestWork, serviceAccountName string, serviceAccountNamespace string) {
	if &manifestWork.Spec == nil {
		manifestWork.Spec = workv1.ManifestWorkSpec{}
	}
	manifestWork.Spec.Executor = &workv1.ManifestWorkExecutor{
		Subject: workv1.ManifestWorkExecutorSubject{
			Type: workv1.ExecutorSubjectTypeServiceAccount,
			ServiceAccount: &workv1.ManifestWorkSubjectServiceAccount{
				Name:      serviceAccountName,
				Namespace: serviceAccountNamespace,
			},
		},
	}
}

// Tries to parse an OAuthClient secret string from the first Spec.Workload.Manifests[] item in
// manifestWork. Returns an error if no Workload exists, no Manifests exist, or the first Manifest
// item cannot be parsed into an OAuthClient.
func GetOAuthClientSecret(manifestWork *workv1.ManifestWork) (string, error) {
	if &manifestWork.Spec.Workload == nil {
		return "", errors.New("Unable to parse OAuthClient from ManifestWork. No workload.")
	}

	if &manifestWork.Spec.Workload.Manifests == nil || len(manifestWork.Spec.Workload.Manifests) == 0 {
		return "", errors.New("Unable to parse OAuthClient from ManifestWork. No manifests.")
	}

	if len(manifestWork.Spec.Workload.Manifests[0].Raw) == 0 {
		return "", errors.New("Unable to parse OAuthClient from ManifestWork. Manifest is empty.")
	}

	oauthClient, err := oauthclientsub.ReadOAuthClientV1([]byte(manifestWork.Spec.Workload.Manifests[0].Raw))
	if err != nil {
		klog.V(4).Infof("Unable to parse OAuthClient from ManifestWork: %v", err)
		return "", err
	}
	secretString := oauthclientsub.GetSecretString(oauthClient)
	return secretString, nil
}
