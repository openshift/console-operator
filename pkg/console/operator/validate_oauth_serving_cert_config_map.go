package operator

import (
	"context"
	"fmt"

	// kube
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	// operator
	"github.com/openshift/console-operator/pkg/api"
)

func (co *consoleOperator) ValidateOAuthServingCertConfigMap(ctx context.Context) (oauthServingCert *corev1.ConfigMap, reason string, err error) {
	oauthServingCertConfigMap, err := co.targetNSConfigMapLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(api.OAuthServingCertConfigMapName)
	if err != nil {
		klog.V(4).Infoln("oauth-serving-cert configmap not found")
		return nil, "FailedGet", fmt.Errorf("oauth-serving-cert configmap not found")
	}

	_, caBundle := oauthServingCertConfigMap.Data["ca-bundle.crt"]
	if !caBundle {
		return nil, "MissingOAuthServingCertBundle", fmt.Errorf("oauth-serving-cert configmap is missing ca-bundle.crt data")
	}
	return oauthServingCertConfigMap, "", nil
}
