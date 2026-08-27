package secret

import (
	"crypto/sha256"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

const (
	testAES256KeyLen    = 32
	testSHA256BlockSize = sha256.BlockSize // 64
)

func validEncryptionKey() []byte {
	return []byte(randomString(testAES256KeyLen))
}

func validAuthenticationKey() []byte {
	return []byte(randomString(testSHA256BlockSize))
}

func TestResetSessionSecretKeysIfNeeded_FreshInstall(t *testing.T) {
	secret := &corev1.Secret{}

	changed := ResetSessionSecretKeysIfNeeded(secret)

	if !changed {
		t.Error("expected changed=true for fresh install")
	}
	if len(secret.Data["sessionEncryptionKey"]) != testAES256KeyLen {
		t.Errorf("sessionEncryptionKey length = %d, want %d", len(secret.Data["sessionEncryptionKey"]), testAES256KeyLen)
	}
	if len(secret.Data["sessionAuthenticationKey"]) != testSHA256BlockSize {
		t.Errorf("sessionAuthenticationKey length = %d, want %d", len(secret.Data["sessionAuthenticationKey"]), testSHA256BlockSize)
	}
	if len(secret.Data["previousSessionEncryptionKey"]) != 0 {
		t.Error("previousSessionEncryptionKey should be empty on fresh install")
	}
	if len(secret.Data["previousSessionAuthenticationKey"]) != 0 {
		t.Error("previousSessionAuthenticationKey should be empty on fresh install")
	}
}

func TestResetSessionSecretKeysIfNeeded_ValidKeysNoOp(t *testing.T) {
	encKey := validEncryptionKey()
	authKey := validAuthenticationKey()

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"sessionEncryptionKey":     encKey,
			"sessionAuthenticationKey": authKey,
		},
	}

	changed := ResetSessionSecretKeysIfNeeded(secret)

	if changed {
		t.Error("expected changed=false when keys are valid")
	}
	if string(secret.Data["sessionEncryptionKey"]) != string(encKey) {
		t.Error("sessionEncryptionKey should not change when valid")
	}
	if string(secret.Data["sessionAuthenticationKey"]) != string(authKey) {
		t.Error("sessionAuthenticationKey should not change when valid")
	}
}

func TestResetSessionSecretKeysIfNeeded_CorruptedCurrentKey_NoPrevious(t *testing.T) {
	badKey := []byte("too-short")

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"sessionEncryptionKey":     badKey,
			"sessionAuthenticationKey": badKey,
		},
	}

	changed := ResetSessionSecretKeysIfNeeded(secret)

	if !changed {
		t.Error("expected changed=true when current keys are invalid")
	}
	if len(secret.Data["sessionEncryptionKey"]) != testAES256KeyLen {
		t.Errorf("sessionEncryptionKey length = %d, want %d", len(secret.Data["sessionEncryptionKey"]), testAES256KeyLen)
	}
	if len(secret.Data["sessionAuthenticationKey"]) != testSHA256BlockSize {
		t.Errorf("sessionAuthenticationKey length = %d, want %d", len(secret.Data["sessionAuthenticationKey"]), testSHA256BlockSize)
	}
	if string(secret.Data["previousSessionEncryptionKey"]) != string(badKey) {
		t.Error("previousSessionEncryptionKey should contain the old (bad) current key when no previous existed")
	}
	if string(secret.Data["previousSessionAuthenticationKey"]) != string(badKey) {
		t.Error("previousSessionAuthenticationKey should contain the old (bad) current key when no previous existed")
	}
}

func TestResetSessionSecretKeysIfNeeded_CorruptedCurrentKey_ValidPreviousPreserved(t *testing.T) {
	validPrevEnc := []byte("valid-previous-encryption-key!!!")
	validPrevAuth := []byte("valid-previous-authentication-key-that-is-sixty-four-bytes-long!")

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"sessionEncryptionKey":             []byte("bad"),
			"sessionAuthenticationKey":         []byte("bad"),
			"previousSessionEncryptionKey":     validPrevEnc,
			"previousSessionAuthenticationKey": validPrevAuth,
		},
	}

	changed := ResetSessionSecretKeysIfNeeded(secret)

	if !changed {
		t.Error("expected changed=true when current keys are invalid")
	}
	if len(secret.Data["sessionEncryptionKey"]) != testAES256KeyLen {
		t.Errorf("sessionEncryptionKey should be regenerated, got length %d", len(secret.Data["sessionEncryptionKey"]))
	}
	if string(secret.Data["previousSessionEncryptionKey"]) != string(validPrevEnc) {
		t.Error("previousSessionEncryptionKey should be preserved, not overwritten with bad current key")
	}
	if string(secret.Data["previousSessionAuthenticationKey"]) != string(validPrevAuth) {
		t.Error("previousSessionAuthenticationKey should be preserved, not overwritten with bad current key")
	}
}

func TestResetSessionSecretKeysIfNeeded_NilData(t *testing.T) {
	secret := &corev1.Secret{Data: nil}

	changed := ResetSessionSecretKeysIfNeeded(secret)

	if !changed {
		t.Error("expected changed=true for nil data")
	}
	if secret.Data == nil {
		t.Fatal("Data map should be initialized")
	}
	if len(secret.Data["sessionEncryptionKey"]) != testAES256KeyLen {
		t.Errorf("sessionEncryptionKey length = %d, want %d", len(secret.Data["sessionEncryptionKey"]), testAES256KeyLen)
	}
	if len(secret.Data["sessionAuthenticationKey"]) != testSHA256BlockSize {
		t.Errorf("sessionAuthenticationKey length = %d, want %d", len(secret.Data["sessionAuthenticationKey"]), testSHA256BlockSize)
	}
}

func TestResetSessionSecretKeysIfNeeded_KeysAreUnique(t *testing.T) {
	secret1 := &corev1.Secret{}
	secret2 := &corev1.Secret{}

	ResetSessionSecretKeysIfNeeded(secret1)
	ResetSessionSecretKeysIfNeeded(secret2)

	if string(secret1.Data["sessionEncryptionKey"]) == string(secret2.Data["sessionEncryptionKey"]) {
		t.Error("two generated encryption keys should not be identical")
	}
	if string(secret1.Data["sessionAuthenticationKey"]) == string(secret2.Data["sessionAuthenticationKey"]) {
		t.Error("two generated authentication keys should not be identical")
	}
}
