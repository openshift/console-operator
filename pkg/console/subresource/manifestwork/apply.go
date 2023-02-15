package manifestwork

import (
	"context"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workclientv1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

func ApplyManifestWork(
	ctx context.Context,
	client workclientv1.ManifestWorkInterface,
	required *workv1.ManifestWork,
) (*workv1.ManifestWork, error) {
	applied, err := client.Get(ctx, api.ManagedClusterOauthClientManifestWork, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return client.Create(ctx, required, metav1.CreateOptions{})
	}

	if err != nil {
		return nil, err
	}

	// Check that the existing resource matches required, and update if it does not
	existingCopy := applied.DeepCopy()
	specSame := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !specSame || *modified {
		existingCopy.Spec = required.Spec
		applied, err = client.Update(ctx, existingCopy, metav1.UpdateOptions{})
		return nil, err
	}
	return applied, nil
}
