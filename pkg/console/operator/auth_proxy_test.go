package operator

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configlisters "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	consoleerrors "github.com/openshift/console-operator/pkg/console/errors"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/proxyconfig"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	clocktesting "k8s.io/utils/clock/testing"
)

type recordingResourceSyncer struct {
	destination, source resourcesynccontroller.ResourceLocation
	calls               int
	err                 error
}

type failingConfigMapLister struct {
	corelisters.ConfigMapLister
	err error
}

func (l *failingConfigMapLister) ConfigMaps(string) corelisters.ConfigMapNamespaceLister {
	return &failingConfigMapNamespaceLister{err: l.err}
}

type failingConfigMapNamespaceLister struct {
	corelisters.ConfigMapNamespaceLister
	err error
}

func (l *failingConfigMapNamespaceLister) Get(string) (*corev1.ConfigMap, error) {
	return nil, l.err
}

type fakeAuthProxyResolver struct {
	disabled bool
	err      error
	calls    int
}

func (r *fakeAuthProxyResolver) ResolveProxy(operatorConfig *operatorv1.Console) (*proxyconfig.Config, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if r.disabled {
		return nil, nil
	}
	return (proxyconfig.ConsoleProxyResolver{}).ResolveProxy(operatorConfig)
}

func (s *recordingResourceSyncer) SyncConfigMap(destination, source resourcesynccontroller.ResourceLocation) error {
	s.destination, s.source = destination, source
	s.calls++
	return s.err
}

func (s *recordingResourceSyncer) SyncSecret(_, _ resourcesynccontroller.ResourceLocation) error {
	return nil
}

func authProxyTestInputs() (*operatorv1.Console, *configv1.Authentication) {
	operator := &operatorv1.Console{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}}
	operator.Spec.AuthProxy = operatorv1.ConsoleAuthProxyConfig{
		HTTPProxy: "http://component.example:3128", NoProxy: []string{"idp.example", ".svc"},
	}
	operator.Spec.ManagementState = operatorv1.Managed
	operator.Spec.Ingress.ConsoleURL = "https://console.example"
	auth := &configv1.Authentication{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}, Spec: configv1.AuthenticationSpec{
		Type: configv1.AuthenticationTypeOIDC,
		OIDCProviders: []configv1.OIDCProvider{{
			Issuer:      configv1.TokenIssuer{URL: "https://idp.example"},
			OIDCClients: []configv1.OIDCClientConfig{{ComponentNamespace: api.TargetNamespace, ComponentName: api.OpenShiftConsoleName, ClientID: "console"}},
		}},
	}}
	return operator, auth
}

func TestSyncAuthProxyTrustedCAConfigMap(t *testing.T) {
	syncErr := errors.New("sync rule rejected")
	lookupErr := errors.New("configmap lookup failed")
	for _, tc := range []struct {
		name          string
		proxy         *proxyconfig.Config
		ready         bool
		syncErr       error
		lookupErr     error
		wantReason    string
		wantSyncError bool
	}{
		{name: "no override"},
		{name: "deactivation clears CA sync source", ready: true},
		{name: "proxy without custom trust", proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128"}},
		{name: "copied CA ready", proxy: &proxyconfig.Config{TrustedCAName: "proxy-ca"}, ready: true},
		{name: "wait for copied CA", proxy: &proxyconfig.Config{TrustedCAName: "proxy-ca"}, wantReason: "AwaitingSync", wantSyncError: true},
		{name: "sync registration error", proxy: &proxyconfig.Config{TrustedCAName: "proxy-ca"}, syncErr: syncErr, wantReason: "FailedResourceSyncUpdate"},
		{name: "deactivation sync error", syncErr: syncErr, wantReason: "FailedResourceSyncUpdate"},
		{name: "destination lookup error", proxy: &proxyconfig.Config{TrustedCAName: "proxy-ca"}, lookupErr: lookupErr, wantReason: "FailedGetConfigMap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syncer := &recordingResourceSyncer{err: tc.syncErr}
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			if tc.ready {
				if err := indexer.Add(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}}); err != nil {
					t.Fatal(err)
				}
			}
			co := &consoleOperator{resourceSyncer: syncer}
			wantSource := resourcesynccontroller.ResourceLocation{}
			if tc.proxy != nil && tc.proxy.TrustedCAName != "" {
				wantSource = resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConfigNamespace, Name: tc.proxy.TrustedCAName}
				if tc.syncErr == nil {
					co.targetNSConfigMapLister = corelisters.NewConfigMapLister(indexer)
					if tc.lookupErr != nil {
						co.targetNSConfigMapLister = &failingConfigMapLister{err: tc.lookupErr}
					}
				}
			}
			reason, err := co.SyncAuthProxyTrustedCAConfigMap(tc.proxy)
			if reason != tc.wantReason || (err != nil) != (tc.wantReason != "") {
				t.Fatalf("reason/error = %q, %v; want reason %q", reason, err, tc.wantReason)
			}
			if consoleerrors.IsSyncError(err) != tc.wantSyncError {
				t.Fatalf("sync error = %t, want %t: %v", consoleerrors.IsSyncError(err), tc.wantSyncError, err)
			}
			if tc.syncErr != nil && !errors.Is(err, tc.syncErr) {
				t.Fatalf("sync registration error was not preserved: %v", err)
			}
			if tc.lookupErr != nil && !strings.Contains(err.Error(), tc.lookupErr.Error()) {
				t.Fatalf("missing lookup error context: %v", err)
			}
			if syncer.calls != 1 || syncer.destination != (resourcesynccontroller.ResourceLocation{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}) || syncer.source != wantSource {
				t.Fatalf("unexpected sync rule: %#v; want source %#v", syncer, wantSource)
			}
		})
	}
}

func TestAuthProxyDeactivationClearsStatus(t *testing.T) {
	for _, tc := range []struct {
		name              string
		deactivate        func(*configv1.Authentication, *fakeAuthProxyResolver)
		wantResolverCalls int
	}{
		{
			name: "proxy gate disabled",
			deactivate: func(_ *configv1.Authentication, resolver *fakeAuthProxyResolver) {
				resolver.disabled = true
			},
			wantResolverCalls: 2,
		},
		{
			name: "no OIDC clients",
			deactivate: func(auth *configv1.Authentication, _ *fakeAuthProxyResolver) {
				auth.Spec.OIDCProviders[0].OIDCClients = nil
			},
			wantResolverCalls: 1,
		},
		{
			name: "client for another component",
			deactivate: func(auth *configv1.Authentication, _ *fakeAuthProxyResolver) {
				auth.Spec.OIDCProviders[0].OIDCClients[0].ComponentName = "another-component"
			},
			wantResolverCalls: 1,
		},
		{
			name: "client in another namespace",
			deactivate: func(auth *configv1.Authentication, _ *fakeAuthProxyResolver) {
				auth.Spec.OIDCProviders[0].OIDCClients[0].ComponentNamespace = "another-namespace"
			},
			wantResolverCalls: 1,
		},
		{
			name: "no OIDC providers",
			deactivate: func(auth *configv1.Authentication, _ *fakeAuthProxyResolver) {
				auth.Spec.OIDCProviders = nil
			},
			wantResolverCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op, auth := authProxyTestInputs()
			op.Spec.AuthProxy.TrustedCA.Name = "proxy-ca"
			// Leave the OAuth client secret absent to stop reconciliation after CA sync.
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := indexer.Add(auth); err != nil {
				t.Fatal(err)
			}
			target := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			serviceCA := configmapsub.DefaultServiceCAConfigMap(op)
			trustedCA := configmapsub.DefaultTrustedCAConfigMap(op)
			for _, cm := range []*corev1.ConfigMap{serviceCA, trustedCA} {
				if err := target.Add(cm); err != nil {
					t.Fatal(err)
				}
			}
			sessionSecret := secretsub.DefaultSessionSecret(op)
			secretIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			if err := secretIndexer.Add(sessionSecret); err != nil {
				t.Fatal(err)
			}
			featureGateIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := featureGateIndexer.Add(&configv1.FeatureGate{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}}); err != nil {
				t.Fatal(err)
			}
			kubeClient := fake.NewClientset(serviceCA, trustedCA, sessionSecret)
			client := v1helpers.NewFakeOperatorClient(&op.Spec.OperatorSpec, &operatorv1.OperatorStatus{}, nil)
			syncer := &recordingResourceSyncer{}
			resolver := &fakeAuthProxyResolver{}
			co := newTestConsoleOperator(t, false, false)
			co.operatorClient = client
			co.resourceSyncer = syncer
			co.authnConfigLister = configlisters.NewAuthenticationLister(indexer)
			co.targetNSConfigMapLister = corelisters.NewConfigMapLister(target)
			co.managedNSConfigMapLister = corelisters.NewConfigMapLister(target)
			co.featureGateLister = configlisters.NewFeatureGateLister(featureGateIndexer)
			co.nodeLister = corelisters.NewNodeLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))
			co.configMapClient = kubeClient.CoreV1()
			co.secretsClient = kubeClient.CoreV1()
			co.secretsLister = corelisters.NewSecretLister(secretIndexer)
			co.authProxyResolver = resolver
			co.trackables.isOLMDisabled = true
			set := configSet{
				Operator: op, Console: &configv1.Console{}, Ingress: &configv1.Ingress{},
				Infrastructure: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{APIServerURL: "https://api.example:6443"}},
			}
			recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
			syncContext := factory.NewSyncContext("test", recorder)
			if err := co.handleSync(t.Context(), syncContext, set); !consoleerrors.IsSyncError(err) {
				t.Fatalf("expected unavailable proxy CA: %v", err)
			}
			_, status, _, err := client.GetOperatorState()
			if err != nil {
				t.Fatal(err)
			}
			if condition := v1helpers.FindOperatorCondition(status.Conditions, "AuthProxyTrustedCASyncProgressing"); condition == nil || condition.Status != operatorv1.ConditionTrue || condition.Reason != "AwaitingSync" {
				t.Fatalf("missing progressing condition: %#v", status.Conditions)
			}
			tc.deactivate(auth, resolver)
			if err := indexer.Update(auth); err != nil {
				t.Fatal(err)
			}
			if err := co.handleSync(t.Context(), syncContext, set); !apierrors.IsNotFound(err) {
				t.Fatalf("expected later unavailable client secret: %v", err)
			}
			if resolver.calls != tc.wantResolverCalls {
				t.Fatalf("proxy resolver calls = %d, want %d", resolver.calls, tc.wantResolverCalls)
			}
			_, status, _, err = client.GetOperatorState()
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"AuthProxyTrustedCASyncDegraded", "AuthProxyTrustedCASyncProgressing"} {
				condition := v1helpers.FindOperatorCondition(status.Conditions, name)
				if condition == nil || condition.Status != operatorv1.ConditionFalse {
					t.Fatalf("stale condition: %#v", condition)
				}
			}
			if syncer.source != (resourcesynccontroller.ResourceLocation{}) {
				t.Fatal("deactivation retained CA sync source")
			}
		})
	}
}

func TestAuthProxyManagementState(t *testing.T) {
	for _, state := range []operatorv1.ManagementState{operatorv1.Unmanaged, operatorv1.Removed} {
		t.Run(string(state), func(t *testing.T) {
			op, _ := authProxyTestInputs()
			op.Spec.ManagementState = state
			kubeClient := fake.NewClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: api.AuthProxyCAConfigMapName, Namespace: api.TargetNamespace}})
			syncer := &recordingResourceSyncer{}
			co := &consoleOperator{resourceSyncer: syncer, configMapClient: kubeClient.CoreV1(), secretsClient: kubeClient.CoreV1(), deploymentClient: kubeClient.AppsV1(),
				operatorClient: v1helpers.NewFakeOperatorClient(&op.Spec.OperatorSpec, &operatorv1.OperatorStatus{}, nil)}
			recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
			if err := co.handleSync(t.Context(), factory.NewSyncContext("test", recorder), configSet{Operator: op}); err != nil {
				t.Fatal(err)
			}
			if state == operatorv1.Unmanaged {
				if len(kubeClient.Actions()) != 0 || syncer.calls != 0 {
					t.Fatal("unmanaged reconciliation wrote resources")
				}
			} else {
				if _, err := kubeClient.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.AuthProxyCAConfigMapName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
					t.Fatal("Removed state retained proxy CA")
				}
				if syncer.calls != 1 || syncer.source != (resourcesynccontroller.ResourceLocation{}) {
					t.Fatal("Removed state retained CA sync source")
				}
			}
		})
	}
}

func TestAuthProxyCARotationDoesNotRollout(t *testing.T) {
	op, _ := authProxyTestInputs()
	op.Spec.AuthProxy.TrustedCA.Name = "proxy-ca"
	target := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: api.AuthProxyCAConfigMapName, Namespace: api.TargetNamespace, ResourceVersion: "1"}, Data: map[string]string{api.AuthProxyCAFileName: "initial bundle"}}
	if err := target.Add(cm); err != nil {
		t.Fatal(err)
	}
	kubeClient := fake.NewClientset()
	co := &consoleOperator{resourceSyncer: &recordingResourceSyncer{}, targetNSConfigMapLister: corelisters.NewConfigMapLister(target), deploymentClient: kubeClient.AppsV1(), authProxyResolver: proxyconfig.ConsoleProxyResolver{}}
	inputs := newSyncDeploymentInputs()
	recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
	build := func() *appsv1.Deployment {
		t.Helper()
		proxy, err := co.authProxyResolver.ResolveProxy(op)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := co.SyncAuthProxyTrustedCAConfigMap(proxy); err != nil {
			t.Fatal(err)
		}
		inputs.authProxy = proxy
		dep, _, err := inputs.callSyncDeployment(co, recorder)
		if err != nil {
			t.Fatal(err)
		}
		return dep
	}
	before := build().Spec.Template.DeepCopy()
	for _, bundle := range []string{"rotated bundle", "invalid bundle", "recovered bundle"} {
		cm = cm.DeepCopy()
		cm.ResourceVersion += "0"
		cm.Data[api.AuthProxyCAFileName] = bundle
		if err := target.Update(cm); err != nil {
			t.Fatal(err)
		}
		kubeClient.ClearActions()
		if after := build(); !reflect.DeepEqual(before, &after.Spec.Template) {
			t.Fatal("CA contents or resource version changed Pod template")
		}
		for _, action := range kubeClient.Actions() {
			if action.GetResource().Resource == "deployments" && (action.GetVerb() == "update" || action.GetVerb() == "patch") {
				t.Fatal("CA rotation updated the Deployment")
			}
		}
	}
	op.Spec.AuthProxy.TrustedCA.Name = "replacement-ca"
	if after := build(); reflect.DeepEqual(before, &after.Spec.Template) {
		t.Fatal("CA reference change did not update Pod template")
	}
}
