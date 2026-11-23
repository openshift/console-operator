package oidcsetup

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

func TestSyncAuthTypeOIDCUsesConfiguredSourceSecret(t *testing.T) {
	const sourceSecretName = "custom-oidc-client-secret"

	tests := []struct {
		name          string
		sourceSecrets []*corev1.Secret
		wantErr       string
	}{
		{
			name: "configured source secret exists",
			sourceSecrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceSecretName,
					Namespace: api.OpenShiftConfigNamespace,
				},
			}},
		},
		{
			name:    "configured source secret is missing",
			wantErr: `failed to get OIDC client secret "custom-oidc-client-secret" from namespace "openshift-config"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentsub.ConsoleOauthConfigName,
				Namespace:       api.OpenShiftConsoleNamespace,
				ResourceVersion: "target-secret-version",
			}}
			deployment := availableDeployment(map[string]string{
				"console.openshift.io/oauth-secret-version": targetSecret.ResourceVersion,
			})

			controller := &oidcSetupController{
				configSecretsLister:       secretLister(t, tt.sourceSecrets...),
				targetNSSecretsLister:     secretLister(t, targetSecret),
				targetNSConfigMapLister:   configMapLister(t),
				targetNSDeploymentsLister: deploymentLister(t, deployment),
				authStatusHandler:         status.NewAuthStatusHandler(nil, api.OpenShiftConsoleName, api.TargetNamespace, api.OpenShiftConsoleOperator),
			}

			err := controller.syncAuthTypeOIDC(context.Background(), oidcAuthentication(sourceSecretName, ""), nil, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("syncAuthTypeOIDC() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("syncAuthTypeOIDC() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCheckClientConfigStatusUsesCustomCAContentHash(t *testing.T) {
	const customCAName = "custom-oidc-ca"

	targetSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:            deploymentsub.ConsoleOauthConfigName,
		Namespace:       api.OpenShiftConsoleNamespace,
		ResourceVersion: "target-secret-version",
	}}
	customCA := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            customCAName,
			Namespace:       api.OpenShiftConsoleNamespace,
			ResourceVersion: "ca-resource-version",
		},
		Data: map[string]string{"ca-bundle.crt": "test-ca-bundle"},
	}

	tests := []struct {
		name             string
		customCARevision string
		wantValid        bool
		wantMessage      string
	}{
		{
			name:             "content hash matches despite different resource version",
			customCARevision: deploymentsub.ConfigMapContentHash(customCA),
			wantValid:        true,
		},
		{
			name:             "resource version does not satisfy content hash annotation",
			customCARevision: customCA.ResourceVersion,
			wantMessage:      "OIDC provider CA version not up to date in current deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := availableDeployment(map[string]string{
				"console.openshift.io/oauth-secret-version":          targetSecret.ResourceVersion,
				"console.openshift.io/authn-ca-trust-config-version": tt.customCARevision,
			})
			controller := &oidcSetupController{
				targetNSSecretsLister:     secretLister(t, targetSecret),
				targetNSConfigMapLister:   configMapLister(t, customCA),
				targetNSDeploymentsLister: deploymentLister(t, deployment),
			}

			valid, message, err := controller.checkClientConfigStatus(oidcAuthentication("source-secret", customCAName))
			if err != nil {
				t.Fatalf("checkClientConfigStatus() unexpected error: %v", err)
			}
			if valid != tt.wantValid || message != tt.wantMessage {
				t.Fatalf("checkClientConfigStatus() = (%t, %q), want (%t, %q)", valid, message, tt.wantValid, tt.wantMessage)
			}
		})
	}
}

func TestCheckClientConfigStatusWaitsForTargetSecret(t *testing.T) {
	controller := &oidcSetupController{
		targetNSSecretsLister:     secretLister(t),
		targetNSDeploymentsLister: deploymentLister(t, availableDeployment(nil)),
	}

	valid, message, err := controller.checkClientConfigStatus(oidcAuthentication("source-secret", ""))
	if err != nil {
		t.Fatalf("checkClientConfigStatus() unexpected error: %v", err)
	}
	wantMessage := `waiting for OIDC client secret "console-oauth-config" to be synced into namespace "openshift-console"`
	if valid || message != wantMessage {
		t.Fatalf("checkClientConfigStatus() = (%t, %q), want (false, %q)", valid, message, wantMessage)
	}
}

func TestCheckClientConfigStatusWrapsUnexpectedTargetSecretError(t *testing.T) {
	unexpectedErr := errors.New("cache read failed")
	controller := &oidcSetupController{
		targetNSSecretsLister:     errorSecretLister{err: unexpectedErr},
		targetNSDeploymentsLister: deploymentLister(t, availableDeployment(nil)),
	}

	_, _, err := controller.checkClientConfigStatus(oidcAuthentication("source-secret", ""))
	wantErr := `failed to get synced OIDC client secret "console-oauth-config" from namespace "openshift-console"`
	if err == nil || !strings.Contains(err.Error(), wantErr) || !errors.Is(err, unexpectedErr) {
		t.Fatalf("checkClientConfigStatus() error = %v, want error containing %q", err, wantErr)
	}
}

func oidcAuthentication(clientSecretName, customCAName string) *configv1.Authentication {
	return &configv1.Authentication{
		Spec: configv1.AuthenticationSpec{
			Type: configv1.AuthenticationTypeOIDC,
			OIDCProviders: []configv1.OIDCProvider{{
				Name: "test-provider",
				Issuer: configv1.TokenIssuer{
					URL:                  "https://issuer.example.com",
					CertificateAuthority: configv1.ConfigMapNameReference{Name: customCAName},
				},
				OIDCClients: []configv1.OIDCClientConfig{{
					ComponentName:      api.OpenShiftConsoleName,
					ComponentNamespace: api.TargetNamespace,
					ClientID:           "test-client",
					ClientSecret:       configv1.SecretNameReference{Name: clientSecretName},
				}},
			}},
		},
	}
}

func availableDeployment(annotations map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        api.OpenShiftConsoleDeploymentName,
			Namespace:   api.OpenShiftConsoleNamespace,
			Generation:  1,
			Annotations: annotations,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas:  1,
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			Replicas:           1,
		},
	}
}

func secretLister(t *testing.T, secrets ...*corev1.Secret) corev1listers.SecretLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, secret := range secrets {
		if err := indexer.Add(secret); err != nil {
			t.Fatalf("failed to add Secret to test indexer: %v", err)
		}
	}
	return corev1listers.NewSecretLister(indexer)
}

type errorSecretLister struct {
	err error
}

func (l errorSecretLister) List(labels.Selector) ([]*corev1.Secret, error) {
	return nil, l.err
}

func (l errorSecretLister) Secrets(string) corev1listers.SecretNamespaceLister {
	return errorSecretNamespaceLister{err: l.err}
}

type errorSecretNamespaceLister struct {
	err error
}

func (l errorSecretNamespaceLister) List(labels.Selector) ([]*corev1.Secret, error) {
	return nil, l.err
}

func (l errorSecretNamespaceLister) Get(string) (*corev1.Secret, error) {
	return nil, l.err
}

func configMapLister(t *testing.T, configMaps ...*corev1.ConfigMap) corev1listers.ConfigMapLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, configMap := range configMaps {
		if err := indexer.Add(configMap); err != nil {
			t.Fatalf("failed to add ConfigMap to test indexer: %v", err)
		}
	}
	return corev1listers.NewConfigMapLister(indexer)
}

func deploymentLister(t *testing.T, deployments ...*appsv1.Deployment) appsv1listers.DeploymentLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, deployment := range deployments {
		if err := indexer.Add(deployment); err != nil {
			t.Fatalf("failed to add Deployment to test indexer: %v", err)
		}
	}
	return appsv1listers.NewDeploymentLister(indexer)
}
