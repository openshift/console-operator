package operator

import (
	"fmt"

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
	)
	for _, logo := range operatorConfig.Spec.Customization.Logos {
		for _, theme := range logo.Themes {
			logoToSync := theme.Source.ConfigMap
			err, reason = co.updateCustomLogoSyncSource(logoToSync)
			if err != nil {
				if aggregatedError == nil {
					aggregatedError = fmt.Errorf("One or more errors were encountered while syncing custom logos:\n  - %v, %s", logoToSync, err.Error())
				} else {
					aggregatedError = fmt.Errorf("%s\n  - %v, %s", aggregatedError.Error(), logoToSync, err.Error())
				}
			}
		}
	}
	if aggregatedError != nil {
		return aggregatedError, reason
	}
	return nil, ""
}

// TODO remove deprecated CustomLogoFile API
func (co *consoleOperator) SyncCustomLogoConfigMap(operatorConfig *operatorv1.Console) (error, string) {
	var customLogoRef = operatorv1.ConfigMapFileReference(operatorConfig.Spec.Customization.CustomLogoFile)
	return co.updateCustomLogoSyncSource(&customLogoRef)
}

// on each pass of the operator sync loop, we need to check the
// operator config for a custom logo.  If this has been set, then
// we notify the resourceSyncer that it needs to start watching this
// configmap in its own sync loop.  Note that the resourceSyncer's actual
// sync loop will run later.  Our operator is waiting to receive
// the copied configmap into the console namespace for a future
// sync loop to mount into the console deployment.
func (c *consoleOperator) updateCustomLogoSyncSource(cmRef *operatorv1.ConfigMapFileReference) (error, string) {
	// validate first, to avoid a broken volume mount & a crashlooping console
	err, reason := c.validateCustomLogo(cmRef)
	if err != nil {
		return err, reason
	}

	source := resourcesynccontroller.ResourceLocation{}
	logoConfigMapName := cmRef.Name

	if logoConfigMapName != "" {
		source.Name = logoConfigMapName
		source.Namespace = api.OpenShiftConfigNamespace
	}
	// if no custom logo provided, sync an empty source to delete
	err = c.resourceSyncer.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConsoleNamespace, Name: cmRef.Name},
		source,
	)
	if err != nil {
		return err, "FailedResourceSync"
	}

	return nil, ""
}

func (co *consoleOperator) validateCustomLogo(logoFileRef *operatorv1.ConfigMapFileReference) (err error, reason string) {
	logoConfigMapName := logoFileRef.Name
	logoImageKey := logoFileRef.Key

	if (len(logoConfigMapName) == 0) != (len(logoImageKey) == 0) {
		klog.V(4).Infoln("custom logo filename or key have not been set")
		return customerrors.NewCustomLogoError("either custom logo filename or key have not been set"), "KeyOrFilenameInvalid"
	}

	// fine if nothing set, but don't mount it
	if len(logoConfigMapName) == 0 {
		klog.V(4).Infoln("no custom logo configured")
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
		klog.V(4).Infoln("custom logo file exists but no image provided")
		return customerrors.NewCustomLogoError("custom logo file exists but no image provided"), "NoImageProvided"
	}

	klog.V(4).Infof("custom logo %s ok to mount", logoConfigMapName)
	return nil, ""
}
