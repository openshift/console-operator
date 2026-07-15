package oauthclient

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oauthv1 "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeRegisterConsoleFromOAuthClient(t *testing.T) {
	type args struct {
		client *oauthv1.OAuthClient
	}
	tests := []struct {
		name string
		args args
		want *oauthv1.OAuthClient
	}{
		{
			name: "Test - Deregister Console from Oauth Client",
			args: args{
				client: &oauthv1.OAuthClient{
					TypeMeta:                            v1.TypeMeta{},
					ObjectMeta:                          v1.ObjectMeta{},
					Secret:                              "TestSecret1",
					AdditionalSecrets:                   nil,
					RespondWithChallenges:               false,
					RedirectURIs:                        []string{"Test"},
					GrantMethod:                         "",
					ScopeRestrictions:                   nil,
					AccessTokenMaxAgeSeconds:            nil,
					AccessTokenInactivityTimeoutSeconds: nil,
				},
			},
			want: &oauthv1.OAuthClient{
				TypeMeta:                            v1.TypeMeta{},
				ObjectMeta:                          v1.ObjectMeta{},
				Secret:                              "shouldBeRandom",
				AdditionalSecrets:                   nil,
				RespondWithChallenges:               false,
				RedirectURIs:                        []string{},
				GrantMethod:                         "",
				ScopeRestrictions:                   nil,
				AccessTokenMaxAgeSeconds:            nil,
				AccessTokenInactivityTimeoutSeconds: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeRegisterConsoleFromOAuthClient(tt.args.client)
			if diff := deep.Equal(got.RedirectURIs, tt.want.RedirectURIs); got.Secret == tt.want.Secret || diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *oauthv1.OAuthClient
	}{
		{
			name: "Test Stub oauth client",
			want: &oauthv1.OAuthClient{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: api.OAuthClientName,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Stub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestSetRedirectURI(t *testing.T) {
	type args struct {
		client *oauthv1.OAuthClient
		host   string
	}
	tests := []struct {
		name string
		args args
		want *oauthv1.OAuthClient
	}{
		{
			name: "Test set redirect URI",
			args: args{
				client: &oauthv1.OAuthClient{
					TypeMeta:                            metav1.TypeMeta{},
					ObjectMeta:                          metav1.ObjectMeta{},
					Secret:                              "",
					AdditionalSecrets:                   nil,
					RespondWithChallenges:               false,
					RedirectURIs:                        nil,
					GrantMethod:                         "",
					ScopeRestrictions:                   nil,
					AccessTokenMaxAgeSeconds:            nil,
					AccessTokenInactivityTimeoutSeconds: nil,
				},
				host: "example.com",
			},
			want: &oauthv1.OAuthClient{
				TypeMeta:                            metav1.TypeMeta{},
				ObjectMeta:                          metav1.ObjectMeta{},
				Secret:                              "",
				AdditionalSecrets:                   nil,
				RespondWithChallenges:               false,
				RedirectURIs:                        []string{"https://example.com/auth/callback"},
				GrantMethod:                         "",
				ScopeRestrictions:                   nil,
				AccessTokenMaxAgeSeconds:            nil,
				AccessTokenInactivityTimeoutSeconds: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(SetRedirectURI(tt.args.client, tt.args.host), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestSetRedirectURIs(t *testing.T) {
	tests := []struct {
		name     string
		hosts    []string
		wantURIs []string
	}{
		{
			name:     "single host",
			hosts:    []string{"console.example.com"},
			wantURIs: []string{"https://console.example.com/auth/callback"},
		},
		{
			name:     "multiple hosts",
			hosts:    []string{"console.example.com", "console-alt.example.com", "console.internal.example.com:8443"},
			wantURIs: []string{"https://console.example.com/auth/callback", "https://console-alt.example.com/auth/callback", "https://console.internal.example.com:8443/auth/callback"},
		},
		{
			name:     "empty hosts",
			hosts:    []string{},
			wantURIs: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &oauthv1.OAuthClient{}
			SetRedirectURIs(client, tt.hosts)
			if diff := deep.Equal(client.RedirectURIs, tt.wantURIs); diff != nil {
				t.Errorf("SetRedirectURIs() mismatch: %v", diff)
			}
		})
	}
}

func TestRegisterConsoleToOAuthClientWithAdditionalHosts(t *testing.T) {
	client := &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: api.OAuthClientName},
	}
	result := RegisterConsoleToOAuthClient(client, "console.example.com", "secret123", "console-alt.example.com", "console.internal.example.com")
	wantURIs := []string{
		"https://console.example.com/auth/callback",
		"https://console-alt.example.com/auth/callback",
		"https://console.internal.example.com/auth/callback",
	}
	if diff := deep.Equal(result.RedirectURIs, wantURIs); diff != nil {
		t.Errorf("RegisterConsoleToOAuthClient() redirect URIs mismatch: %v", diff)
	}
	if result.Secret != "secret123" {
		t.Errorf("expected secret %q, got %q", "secret123", result.Secret)
	}
}

func TestRegisterConsoleToOAuthClientNoAdditionalHosts(t *testing.T) {
	client := &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: api.OAuthClientName},
	}
	result := RegisterConsoleToOAuthClient(client, "console.example.com", "secret123")
	wantURIs := []string{"https://console.example.com/auth/callback"}
	if diff := deep.Equal(result.RedirectURIs, wantURIs); diff != nil {
		t.Errorf("RegisterConsoleToOAuthClient() redirect URIs mismatch: %v", diff)
	}
}
