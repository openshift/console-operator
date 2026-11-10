package operator

import (
	"fmt"
	"path"

	"gopkg.in/yaml.v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
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

// authProxyCAPlan describes what a single sync may do about the auth proxy CA.
//
// The rollout is staged so that no Pod ever references a file it does not have:
//
//	Adding:   sync the CA copy -> mount it in the Deployment -> reference its path in the config.
//	Removing: drop the path from the config -> unmount it -> remove the CA copy.
//
// Every step waits for the previous one to be visible in the cluster, so each
// sync only needs to look at the current state, not remember where it was.
//
// The state is read from the listers. They may lag behind what the previous sync
// wrote, which is bridged by what this process remembers having written: the
// Console configuration it applied last and the Deployment generation it applied
// last. A Deployment that is older than that tells nothing about the Pods.
type authProxyCAPlan struct {
	// configProxy is the proxy to render into the Console configuration. It
	// references the CA file only when every Pod is known to have it mounted.
	configProxy *proxyconfig.Config
	// mount tells whether the Deployment mounts the CA volume. While the CA is
	// configured, it is only acted upon after syncCopy succeeded: the sync returns
	// early while the copy is missing, so the Deployment is never updated to mount
	// a ConfigMap that is not there. While the CA is being removed, it is only set
	// when the copy exists.
	mount bool
	// syncCopy tells whether the CA copy is to be registered and awaited. A sync
	// error (the copy is not there yet) stops the sync before the config and Deployment.
	syncCopy bool
	// removeCopy tells whether the CA copy may be unregistered, i.e. no Pod can mount it.
	removeCopy bool
}

func (c *consoleOperator) planAuthProxyCA(proxy *proxyconfig.Config) (authProxyCAPlan, error) {
	plan := authProxyCAPlan{configProxy: proxy}

	published, err := c.authProxyCAPublished()
	if err != nil {
		return authProxyCAPlan{}, err
	}
	deployment, deploymentMissing, err := c.observedDeployment()
	if err != nil {
		return authProxyCAPlan{}, err
	}

	if proxy != nil && proxy.TrustedCAName != "" {
		plan.syncCopy = true
		plan.mount = true
		// Once published the path is kept, so unrelated rollouts don't flip it.
		if !published && !deploymentsub.RolledOutWithVolume(deployment, api.AuthProxyCAConfigMapName) {
			withoutCA := *proxy
			withoutCA.TrustedCAName = ""
			plan.configProxy = &withoutCA
		}
		return plan, nil
	}

	// The CA is not (or no longer) configured, so the config is rendered without it.
	// While the previously published config still references the file, its Pods
	// need the mount, as long as there is a copy to mount.
	if published {
		_, err := c.targetNSConfigMapLister.ConfigMaps(api.TargetNamespace).Get(api.AuthProxyCAConfigMapName)
		if err == nil {
			plan.mount = true
			return plan, nil
		}
		if !apierrors.IsNotFound(err) {
			return authProxyCAPlan{}, fmt.Errorf("getting authentication proxy CA ConfigMap \"%s/%s\": %w", api.TargetNamespace, api.AuthProxyCAConfigMapName, err)
		}
	}
	// Unmounted by an earlier sync. The copy goes only after that rollout is complete.
	plan.removeCopy = deploymentMissing || deploymentsub.RolledOutWithoutVolume(deployment, api.AuthProxyCAConfigMapName)
	return plan, nil
}

// authProxyCAPublished returns true if the published Console configuration, which
// is what Pods consume, references the mounted proxy CA file. The configuration
// applied last by this process is preferred to the lister, which may not have
// seen it yet. After a restart, there is only the lister, which is synced by then.
func (c *consoleOperator) authProxyCAPublished() (bool, error) {
	if c.lastAppliedConsoleConfig != nil {
		return authProxyCAReferenced(c.lastAppliedConsoleConfig)
	}
	cm, err := c.targetNSConfigMapLister.ConfigMaps(api.TargetNamespace).Get(api.OpenShiftConsoleConfigMapName)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("getting Console configuration \"%s/%s\": %w", api.TargetNamespace, api.OpenShiftConsoleConfigMapName, err)
	}
	return authProxyCAReferenced(cm)
}

// observedDeployment returns the console Deployment from the lister, or whether
// it is missing. A Deployment older than the one this process applied last is
// returned as nil, without being missing: it tells nothing about the template
// the Pods are being created from right now, so no step may depend on it.
func (c *consoleOperator) observedDeployment() (deployment *appsv1.Deployment, missing bool, err error) {
	deployment, err = c.deploymentLister.Deployments(api.TargetNamespace).Get(api.OpenShiftConsoleDeploymentName)
	switch {
	case apierrors.IsNotFound(err):
		// Not seen yet, if this process has already applied it.
		return nil, c.lastAppliedDeploymentGeneration == 0, nil
	case err != nil:
		return nil, false, fmt.Errorf("getting Deployment \"%s/%s\": %w", api.TargetNamespace, api.OpenShiftConsoleDeploymentName, err)
	case deployment.Generation < c.lastAppliedDeploymentGeneration:
		return nil, false, nil
	}
	return deployment, false, nil
}

// authProxyCAReferenced returns true if the Console configuration sets the auth
// proxy CA file to the mounted CA.
func authProxyCAReferenced(cm *corev1.ConfigMap) (bool, error) {
	var config consoleserver.Config
	if err := yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &config); err != nil {
		return false, fmt.Errorf("parsing Console configuration \"%s/%s\": %w", api.TargetNamespace, api.OpenShiftConsoleConfigMapName, err)
	}
	return config.Auth.Proxy != nil && config.Auth.Proxy.TrustedCAFilePath == path.Join(api.AuthProxyCAMountDir, api.AuthProxyCAFileName), nil
}
