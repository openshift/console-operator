package configmap

import (
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/operator/v1"
)

type CustomLogoRef struct {
	Location  v1.LogoType
	Theme     v1.ThemeType
	File      configv1.ConfigMapFileReference
	OkToMount bool
}

// borrowed from the image package
// image.RegisterFormat()
var commonImageHeaders = []string{
	"\xff\xd8\xff",      // "image/jpeg"
	"\x89PNG\r\n\x1a\n", // "image/png"
	"GIF87a",            // "image/gif"
	"GIF89a",            // "image/gif"
}

func FileNameOrKeyInconsistentlySet(operatorConfig *v1.Console) bool {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	logoImageKey := operatorConfig.Spec.Customization.CustomLogoFile.Key
	return (len(logoConfigMapName) == 0) != (len(logoImageKey) == 0)
}

func FileNameNotSet(operatorConfig *v1.Console) bool {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	return len(logoConfigMapName) == 0
}

func IsRemoved(operatorConfig *v1.Console) bool {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	logoImageKey := operatorConfig.Spec.Customization.CustomLogoFile.Key
	return logoConfigMapName == "" && logoImageKey == ""
}
