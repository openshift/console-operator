package operator

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/proxyconfig"
)

func (c *consoleOperator) SyncAuthProxyTrustedCAConfigMap(proxy *proxyconfig.Config) (string, error) {
	destination := resourcesynccontroller.ResourceLocation{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}
	source := resourcesynccontroller.ResourceLocation{}
	if proxy != nil && proxy.TrustedCAName != "" {
		source = resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConfigNamespace, Name: proxy.TrustedCAName}
	}
	if err := c.resourceSyncer.SyncConfigMap(destination, source); err != nil {
		return "FailedResourceSyncUpdate", fmt.Errorf("registering authentication proxy CA sync: %w", err)
	}

	if source.Name != "" {
		if _, err := c.targetNSConfigMapLister.ConfigMaps(destination.Namespace).Get(destination.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return "AwaitingSync", errors.NewSyncError(fmt.Sprintf("waiting for authentication proxy CA ConfigMap %s/%s", destination.Namespace, destination.Name))
			}
			return "FailedGetConfigMap", fmt.Errorf("getting authentication proxy CA ConfigMap %s/%s: %v", destination.Namespace, destination.Name, err)
		}
	}
	return "", nil
}
