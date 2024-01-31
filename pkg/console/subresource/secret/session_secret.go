package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func DefaultSessionSecret(cr *operatorv1.Console) *corev1.Secret {
	meta := util.SharedMeta()
	meta.Name = api.SessionSecretName

	secret := &corev1.Secret{
		ObjectMeta: meta,
	}
	util.AddOwnerRef(secret, util.OwnerRefFrom(cr))

	ResetSessionSecretKeysIfNeeded(secret)
	return secret
}

func ResetSessionSecretKeysIfNeeded(secret *corev1.Secret) bool {
	const (
		sha256KeyLenBytes = sha256.BlockSize // max key size with HMAC SHA256
		aes256KeyLenBytes = 32               // max key size with AES (AES-256)
	)

	var changed bool

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	if len(secret.Data["sessionEncryptionKey"]) != aes256KeyLenBytes {
		secret.Data["sessionEncryptionKey"] = []byte(randomString(aes256KeyLenBytes))
		changed = true
	}

	if len(secret.Data["sessionAuthenticationKey"]) != sha256KeyLenBytes {
		secret.Data["sessionAuthenticationKey"] = []byte(randomString(sha256KeyLenBytes))
		changed = true
	}

	return changed
}

// needs to be in lib-go
func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err) // rand should never fail
	}
	return b
}

// randomString uses RawURLEncoding to ensure we do not get / characters or trailing ='s
func randomString(size int) string {
	// each byte (8 bits) gives us 4/3 base64 (6 bits) characters
	// we account for that conversion and add one to handle truncation
	b64size := base64.RawURLEncoding.DecodedLen(size) + 1
	// trim down to the original requested size since we added one above
	return base64.RawURLEncoding.EncodeToString(randomBytes(b64size))[:size]
}
