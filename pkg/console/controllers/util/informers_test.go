package util

import (
	"context"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	fakeconfig "github.com/openshift/client-go/config/clientset/versioned/fake"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	fakeoauthclient "github.com/openshift/client-go/oauth/clientset/versioned/fake"
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"
	"github.com/openshift/library-go/pkg/operator/events"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestEnsureRunningAndStop(t *testing.T) {
	testCtx, testCancel := context.WithCancel(context.TODO())
	defer testCancel()

	testOAuthClient := &oauthv1.OAuthClient{}
	testClient := fakeoauthclient.NewSimpleClientset(testOAuthClient)
	testInformer := oauthinformers.NewSharedInformerFactory(testClient, 0).Oauth().V1().OAuthClients()

	informerSwitch := InformerWithSwitch{
		parentCtx: testCtx,
		informer:  testInformer,
	}

	informerSwitch.ensureRunning()
	err := wait.PollUntilContextTimeout(testCtx, 100*time.Millisecond, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		return informerSwitch.Informer().HasSynced(), nil
	})
	if err != nil {
		t.Errorf("unexpected error while waiting for informer to sync: %v", err)
	}

	if informerSwitch.runCtx == nil {
		t.Error("EnsureRunning: runCtx is nil when it should be non-nil")
	}

	if informerSwitch.stopFunc == nil {
		t.Error("EnsureRunning: stopFunc is nil when it should be non-nil")
	}

	if informerSwitch.Informer().IsStopped() {
		t.Error("EnsureRunning: informer is stopped when it should be started")
	}

	informerSwitch.stop()
	err = wait.PollUntilContextTimeout(testCtx, 100*time.Millisecond, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		return informerSwitch.Informer().IsStopped(), nil
	})
	if err != nil {
		t.Errorf("unexpected error while waiting for informer to stop: %v", err)
	}

	if informerSwitch.runCtx != nil {
		t.Error("Stop: runCtx is not nil when it should be nil")
	}

	if informerSwitch.stopFunc != nil {
		t.Error("Stop: stopFunc is not nil when it should be nil")
	}

	if !informerSwitch.Informer().IsStopped() {
		t.Error("Stop: informer is started when it should be stopped")
	}
}

func TestSync(t *testing.T) {
	tests := []struct {
		name           string
		authType       configv1.AuthenticationType
		authConfigName string
		expectRunning  bool
		expectError    bool
	}{
		{"sync fails for unknown auth type", configv1.AuthenticationType("unknown"), "cluster", false, true},
		{"sync fails for unknown auth object", configv1.AuthenticationTypeIntegratedOAuth, "unknown", false, true},
		{"informer running when auth type IntegratedOAuth", configv1.AuthenticationTypeIntegratedOAuth, "cluster", true, false},
		{"informer running when auth type empty", configv1.AuthenticationType(""), "cluster", true, false},
		{"informer not running when auth type OIDC", configv1.AuthenticationTypeOIDC, "cluster", false, false},
		// We don't disable auth since the internal OAuth server is not disabled even with auth type 'None'.
		{"informer running when auth type None", configv1.AuthenticationTypeNone, "cluster", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, stopFunc := context.WithCancel(context.TODO())
			defer stopFunc()

			authn := &configv1.Authentication{
				ObjectMeta: v1.ObjectMeta{
					Name: tt.authConfigName,
				},
				Spec: configv1.AuthenticationSpec{
					Type: tt.authType,
				},
			}

			testClient := fakeoauthclient.NewSimpleClientset()
			testAuthnClient := fakeconfig.NewSimpleClientset(authn)
			testAuthnInformer := configinformers.NewSharedInformerFactory(testAuthnClient, 0).Config().V1().Authentications()
			go testAuthnInformer.Informer().Run(ctx.Done())
			err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
				return testAuthnInformer.Informer().HasSynced(), nil
			})
			if err != nil {
				t.Errorf("unexpected error while waiting for authentication informer to sync: %v", err)
			}

			switchedInformer := NewSwitchedInformer(
				ctx,
				testClient,
				0,
				testAuthnInformer,
				events.NewInMemoryRecorder(tt.name, clocktesting.NewFakePassiveClock(time.Now())),
			)

			err = switchedInformer.sync(ctx, nil)
			if tt.expectError != (err != nil) {
				t.Errorf("sync error: want %v; got %v", tt.expectError, err)
			}

			if tt.expectRunning != (switchedInformer.runCtx != nil && switchedInformer.stopFunc != nil) {
				t.Errorf("informer stopped: got %v; want %v", switchedInformer.Informer().IsStopped(), tt.expectRunning)
			}
		})
	}
}
