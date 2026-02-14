package serviceaccount

import (
	corev1 "k8s.io/api/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

func DefaultDownloadsServiceAccount(operatorConfig *operatorv1.Console) *corev1.ServiceAccount {
	serviceAccount := resourceread.ReadServiceAccountV1OrDie(
		bindata.MustAsset("assets/serviceaccounts/downloads-sa.yaml"),
	)
	util.AddOwnerRef(serviceAccount, util.OwnerRefFrom(operatorConfig))
	return serviceAccount
}
