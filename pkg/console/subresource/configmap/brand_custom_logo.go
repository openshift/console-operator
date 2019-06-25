package configmap

import v1 "github.com/openshift/api/operator/v1"

// borrowed from the image package
// image.RegisterFormat()
var commonImageHeaders = []string{
	"\xff\xd8\xff",      // "image/jpeg"
	"\x89PNG\r\n\x1a\n", // "image/png"
	"GIF87a",            // "image/gif"
	"GIF89a",            // "image/gif"
}

// TODO: implement this
// tests if bytes provided are likely an image by reading the
// first set of bytes and checking against some well known
// image headers.
func IsLikelyCommonImageFormat(bytes []byte) bool {
	// we can probably look at how RegisterFormat works:
	// image.RegisterFormat()
	// - uses several funcs such as match() and sniff() that give good
	//   direction for implementing a basic check for image type.
	// - RegisterFormat() is used to register gif,jpeg,png
	return true
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

func LogoImageIsEmpty(image []byte) bool {
	return len(image) == 0
}

func IsRemoved(operatorConfig *v1.Console) bool {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	logoImageKey := operatorConfig.Spec.Customization.CustomLogoFile.Key
	return logoConfigMapName == "" && logoImageKey == ""
}
