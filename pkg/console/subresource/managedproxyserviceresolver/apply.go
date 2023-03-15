package managedproxyserviceresolver

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	proxy "open-cluster-management.io/cluster-proxy/pkg/apis/proxy/v1alpha1"
	proxyclient "open-cluster-management.io/cluster-proxy/pkg/generated/clientset/versioned/typed/proxy/v1alpha1"
)

// Apply merges objectmeta, requires spec.
// If existing.Spec doesn't deeply equal required.Spec, existing is updated to match required.
func ApplyManagedProxyServiceResolver(ctx context.Context, client proxyclient.ManagedProxyServiceResolverInterface, required *proxy.ManagedProxyServiceResolver) error {
	existing, err := client.Get(ctx, required.Name, metav1.GetOptions{})

	// If existing resource isn't found, try to create it
	if apierrors.IsNotFound(err) {
		_, err = client.Create(ctx, required, metav1.CreateOptions{})

		// Create failed, return error
		if err != nil {
			klog.V(4).Infof("failed to create ManagedProxyServiceResolver %q: %v/n", required.Name, err)
			return err
		}

		// Required resource was successfuly created
		return nil
	}

	// Get failed, return error
	if err != nil {
		klog.V(4).Infoln("failed to get ManagedProxyServiceResolver %q: %v/n", required.Name, err)
		return err
	}

	// Check that the existing spec and metadata state match the required state
	existingCopy := existing.DeepCopy()
	specEqual := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)

	// Update if existing has diverged from required
	if *modified || !specEqual {
		existingCopy.Spec = required.Spec
		_, err = client.Update(ctx, existingCopy, metav1.UpdateOptions{})

		// Update failed, return error
		if err != nil {
			klog.V(4).Infof("failed to update ManagedProxyServiceResolver %q: %v/n", required.Name, err)
			return err
		}
	}

	// Update successful and/or required state is present
	return nil
}
