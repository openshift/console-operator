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
	// We must always call SyncConfigMap, even when no proxy is configured,
	// because that actually ensures the ConfigMap gets removed when proxy is removed.
	destination := resourcesynccontroller.ResourceLocation{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}
	source := resourcesynccontroller.ResourceLocation{}
	if proxy != nil && proxy.TrustedCAName != "" {
		sourceConfigMap, err := c.configNSConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(proxy.TrustedCAName)
		if apierrors.IsNotFound(err) {
			return "SourceNotFound", fmt.Errorf("authentication proxy trustedCA ConfigMap %s/%s not found", api.OpenShiftConfigNamespace, proxy.TrustedCAName)
		}
		if err != nil {
			return "FailedGetSource", fmt.Errorf("getting authentication proxy trustedCA ConfigMap %s/%s: %w", api.OpenShiftConfigNamespace, proxy.TrustedCAName, err)
		}
		if _, ok := sourceConfigMap.Data[api.AuthProxyCAFileName]; !ok {
			return "InvalidSource", fmt.Errorf("authentication proxy trustedCA ConfigMap %s/%s is missing required key %q", api.OpenShiftConfigNamespace, proxy.TrustedCAName, api.AuthProxyCAFileName)
		}
		source = resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConfigNamespace, Name: proxy.TrustedCAName}
	}
	if err := c.resourceSyncer.SyncConfigMap(destination, source); err != nil {
		return "FailedResourceSyncUpdate", fmt.Errorf("registering authentication proxy CA sync: %w", err)
	}

	// We block further sync/deployment when the ConfigMap is not yet in place.
	// This is because Console required the volume and actually loads the bundle initially on startup,
	// then starts watching it. This is consistent with what other operators/operands do.
	if source.Name != "" {
		if _, err := c.targetNSConfigMapLister.ConfigMaps(destination.Namespace).Get(destination.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return "AwaitingLocalConfigMapSync", errors.NewSyncError(fmt.Sprintf("waiting for authentication proxy CA ConfigMap %s/%s", destination.Namespace, destination.Name))
			}
			return "FailedGetLocalConfigMap", fmt.Errorf("getting authentication proxy CA ConfigMap %s/%s: %v", destination.Namespace, destination.Name, err)
		}
	}
	return "", nil
}
