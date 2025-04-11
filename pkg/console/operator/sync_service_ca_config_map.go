package operator

import (
	"context"

	// kube
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
)

// apply service-ca configmap
func (co *consoleOperator) SyncServiceCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (consoleCM *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.targetNSConfigMapLister.ConfigMaps(required.Namespace).Get(required.Name)
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		if err == nil {
			klog.V(4).Infoln("service-ca configmap created")
			return actual, true, "", err
		} else {
			return actual, true, "FailedCreate", err
		}
	}
	if err != nil {
		return nil, false, "FailedGet", err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("service-ca configmap exists and is in the correct state")
		return existing, false, "", nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err == nil {
		klog.V(4).Infoln("service-ca configmap updated")
		return actual, true, "", err
	} else {
		return actual, true, "FailedUpdate", err
	}
}
