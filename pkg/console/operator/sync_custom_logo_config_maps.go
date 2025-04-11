package operator

import (
	"fmt"
	"slices"

	// kube
	"k8s.io/klog/v2"

	// openshift

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
)

func (co *consoleOperator) SyncCustomLogos(operatorConfig *operatorv1.Console) (error, string) {
	if operatorConfig.Spec.Customization.CustomLogoFile.Name != "" || operatorConfig.Spec.Customization.CustomLogoFile.Key != "" {
		return co.SyncCustomLogoConfigMap(operatorConfig)
	}

	var (
		aggregatedError error
		err             error
		reason          string
		newSyncedLogos  []string
	)
	for _, logo := range operatorConfig.Spec.Customization.Logos {
		for _, theme := range logo.Themes {
			logoToSync := theme.Source.ConfigMap
			if err, reason = co.validateCustomLogo(logoToSync); err != nil {
				if aggregatedError == nil {
					aggregatedError = fmt.Errorf("error syncing custom logos:  - Invalid config: %v, %s", logoToSync, err.Error())
				} else {
					aggregatedError = fmt.Errorf("%s  - %v, %s", aggregatedError.Error(), logoToSync, err.Error())
				}
			} else {
				newSyncedLogos = append(newSyncedLogos, logoToSync.Name)
			}
		}
	}
	if aggregatedError != nil {
		return aggregatedError, reason
	}
	slices.Sort(newSyncedLogos)
	return co.updateCustomLogoSyncSources(newSyncedLogos)
}

// TODO remove deprecated CustomLogoFile API
func (co *consoleOperator) SyncCustomLogoConfigMap(operatorConfig *operatorv1.Console) (error, string) {
	var customLogoRef = operatorv1.ConfigMapFileReference(operatorConfig.Spec.Customization.CustomLogoFile)
	klog.V(4).Infof("syncing customLogoFile, Name: %s, Key: %s", customLogoRef.Name, customLogoRef.Key)
	err, reason := co.validateCustomLogo(&customLogoRef)
	if err != nil {
		klog.V(4).Infof("failed to sync customLogoFile, %v", err)
		return err, reason
	}
	return co.updateCustomLogoSyncSources([]string{customLogoRef.Name})
}

// on each pass of the operator sync loop, we need to check the
// operator config for custom logos.  If this has been set, then
// we notify the resourceSyncer that it needs to start watching the associated
// configmaps in its own sync loop.  Note that the resourceSyncer's actual
// sync loop will run later.  Our operator is waiting to receive
// the copied configmaps into the console namespace for a future
// sync loop to mount into the console deployment.
func (co *consoleOperator) updateCustomLogoSyncSources(configMapNames []string) (error, string) {
	klog.V(4).Info("syncing custom logo configmap resources")
	klog.V(4).Infof("%#v", configMapNames)

	errors := []string{}
	if len(co.trackables.customLogoConfigMaps) > 0 {
		klog.V(4).Info("unsyncing custom logo configmap resources from previous sync loop...")
		for _, configMapName := range co.trackables.customLogoConfigMaps {
			err := co.updateCustomLogoSyncSource(configMapName, true)
			if err != nil {
				errors = append(errors, err.Error())
			}
		}

		if len(errors) > 0 {
			msg := fmt.Sprintf("error syncing custom logo configmap resources\n%v", errors)
			klog.V(4).Info(msg)
			return fmt.Errorf(msg), "FailedResourceSync"
		}
	}

	if len(configMapNames) > 0 {
		// If the new list of synced configmaps is different than the last sync, we need to update the
		// resource syncer with the new list, and re
		klog.V(4).Infof("syncing new custom logo configmap resources...")
		for _, configMapName := range configMapNames {
			err := co.updateCustomLogoSyncSource(configMapName, false)
			if err != nil {
				errors = append(errors, err.Error())
			}
		}

		if len(errors) > 0 {
			msg := fmt.Sprintf("error syncing custom logo configmap resources:\n%v", errors)
			klog.V(4).Infof(msg)
			return fmt.Errorf(msg), "FailedResourceSync"
		}
	}

	klog.V(4).Info("saving synced custom logo configmap resources for next loop")
	co.trackables.customLogoConfigMaps = configMapNames

	klog.V(4).Info("done")
	return nil, ""
}

func (co *consoleOperator) validateCustomLogo(logoFileRef *operatorv1.ConfigMapFileReference) (err error, reason string) {
	logoConfigMapName := logoFileRef.Name
	logoImageKey := logoFileRef.Key

	if (len(logoConfigMapName) == 0) != (len(logoImageKey) == 0) {
		klog.V(4).Info("custom logo filename or key have not been set")
		return customerrors.NewCustomLogoError("either custom logo filename or key have not been set"), "KeyOrFilenameInvalid"
	}

	// fine if nothing set, but don't mount it
	if len(logoConfigMapName) == 0 {
		klog.V(4).Info("no custom logo configured")
		return nil, ""
	}

	logoConfigMap, err := co.configNSConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(logoConfigMapName)
	// If we 404, the logo file may not have been created yet.
	if err != nil {
		klog.V(4).Infof("failed to get ConfigMap %v, %v", logoConfigMapName, err)
		return customerrors.NewCustomLogoError(fmt.Sprintf("failed to get ConfigMap %v, %v", logoConfigMapName, err)), "FailedGet"
	}

	_, imageDataFound := logoConfigMap.BinaryData[logoImageKey]
	if !imageDataFound {
		_, imageDataFound = logoConfigMap.Data[logoImageKey]
	}
	if !imageDataFound {
		klog.V(4).Info("custom logo file exists but no image provided")
		return customerrors.NewCustomLogoError("custom logo file exists but no image provided"), "NoImageProvided"
	}

	klog.V(4).Infof("custom logo %s ok to mount", logoConfigMapName)
	return nil, ""
}

func (co *consoleOperator) updateCustomLogoSyncSource(targetName string, unsync bool) error {
	source := resourcesynccontroller.ResourceLocation{}
	if !unsync {
		source.Name = targetName
		source.Namespace = api.OpenShiftConfigNamespace
	}

	target := resourcesynccontroller.ResourceLocation{
		Namespace: api.OpenShiftConsoleNamespace,
		Name:      targetName,
	}

	if unsync {
		klog.V(4).Infof("unsyncing %s", targetName)
	} else {
		klog.V(4).Infof("syncing %s", targetName)
	}
	return co.resourceSyncer.SyncConfigMap(target, source)
}
