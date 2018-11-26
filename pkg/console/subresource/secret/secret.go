package secret

import (
	// 3rd
	"github.com/sirupsen/logrus"
	// kube
	corev1 "k8s.io/api/core/v1"
	// openshift
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

const ClientSecretKey = "clientSecret"

func DefaultSecret(cr *v1alpha1.Console, randomBits string) *corev1.Secret {
	logrus.Printf("DefaultSecret() %v", randomBits)

	secret := Stub()
	// TODO: client-go ignores the StringData field. Open a PR to fix this
	//secret.StringData = map[string]string{
	//	ClientSecretKey: randomBits,
	//}
	secret.Data = map[string][]byte{
		ClientSecretKey: []byte(randomBits),
	}

	util.AddOwnerRef(secret, util.OwnerRefFrom(cr))
	return secret
}

func Stub() *corev1.Secret {
	meta := util.SharedMeta()
	meta.Name = deployment.ConsoleOauthConfigName

	secret := &corev1.Secret{
		ObjectMeta: meta,
	}
	return secret
}

func GetSecretString(secret *corev1.Secret) string {
	return string(secret.Data[ClientSecretKey])
}
