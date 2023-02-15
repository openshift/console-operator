package oauthclient

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	oauthscheme "github.com/openshift/client-go/oauth/clientset/versioned/scheme"
)

func ReadOAuthClientV1(objBytes []byte) (*oauthv1.OAuthClient, error) {
	groupVersionKind := oauthv1.GroupVersion.WithKind("OAuthClient")
	resource, _, err := oauthscheme.Codecs.UniversalDecoder().Decode(objBytes, &groupVersionKind, &oauthv1.OAuthClient{})
	if err != nil {
		return nil, err
	}
	return resource.(*oauthv1.OAuthClient), nil
}
