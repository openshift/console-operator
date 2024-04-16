package crd

import (
	"fmt"

	apiexensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiexensionsv1listers "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
)

func AuthnConfigHasOIDCFields(crdLister apiexensionsv1listers.CustomResourceDefinitionLister) (bool, error) {
	authnCRD, err := crdLister.Get("authentications.config.openshift.io")
	if err != nil {
		return false, err
	}

	var authnV1Config *apiexensionsv1.CustomResourceDefinitionVersion
	for _, version := range authnCRD.Spec.Versions {
		if version.Name == "v1" && version.Served && version.Storage {
			authnV1Config = &version
			break
		}
	}

	if authnV1Config == nil {
		return false, fmt.Errorf("authentications.config.openshift.io is not served or stored as v1")
	}

	schema := authnV1Config.Schema.OpenAPIV3Schema
	_, clientsExist := schema.Properties["status"].Properties["oidcClients"]

	return clientsExist, nil

}
