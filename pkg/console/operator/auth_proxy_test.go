package operator

import (
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	clocktesting "k8s.io/utils/clock/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/api"
	consoleerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/proxyconfig"
)

// copiedCAState describes the state of the proxy CA in the console namespace that
// the resource syncer is responsible for copying there. It only matters when a
// custom trust is configured and the sync rule was registered successfully;
// otherwise the operator never reads it (copiedCAIrrelevant).
type copiedCAState int

const (
	copiedCAIrrelevant  copiedCAState = iota // the operator does not read the copied CA in this scenario
	copiedCAPresent                          // the copy already landed in the console namespace
	copiedCAPending                          // the copy has not appeared yet (NotFound)
	copiedCALookupFails                      // reading the copy returns an unexpected error
)

// sourceCAState describes the state of the administrator-supplied proxy CA
// ConfigMap in the openshift-config namespace that SyncAuthProxyTrustedCAConfigMap
// must validate before registering the sync rule. It only matters when a custom
// trust is configured (sourceCAIrrelevant otherwise).
type sourceCAState int

const (
	sourceCAIrrelevant  sourceCAState = iota // the operator does not look up a source in this scenario
	sourceCAValid                            // the source ConfigMap exists and has the required key
	sourceCAMissingKey                       // the source ConfigMap exists but lacks the required key
	sourceCAPending                          // the source ConfigMap does not exist yet (NotFound)
	sourceCALookupFails                      // reading the source ConfigMap returns an unexpected error
)

// TestSyncAuthProxyTrustedCAConfigMap covers the sync rule that copies the
// administrator's proxy CA into the console namespace and the condition reasons
// the operator reports while that copy is pending or failing.
func TestSyncAuthProxyTrustedCAConfigMap(t *testing.T) {
	const caLookupMessage = "the copied CA could not be read"
	const sourceLookupMessage = "the source CA could not be read"
	wantDestination := resourcesynccontroller.ResourceLocation{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}
	configSource := resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConfigNamespace, Name: "proxy-ca"}
	withCustomTrust := &proxyconfig.Config{TrustedCAName: "proxy-ca"}

	for _, tc := range []struct {
		name string

		// inputs
		proxy        *proxyconfig.Config // resolved component proxy; nil means no override
		sourceCA     sourceCAState       // state of the admin-supplied source CA, once a custom trust is configured
		syncRejected bool                // the resource syncer rejects the sync rule
		copiedCA     copiedCAState       // state of the copied CA, once a custom trust is configured

		// outputs
		wantSource      resourcesynccontroller.ResourceLocation // source registered on the sync rule
		wantReason      string
		wantErrContains string // substring the returned error must contain; "" means no error
		wantSyncError   bool   // the error is a retriable sync error
		wantSyncCalls   int    // number of times SyncConfigMap is expected to be called
	}{
		{
			name:          "no override clears the source",
			proxy:         nil,
			wantSource:    resourcesynccontroller.ResourceLocation{},
			wantSyncCalls: 1,
		},
		{
			name:          "proxy without custom trust clears the source",
			proxy:         &proxyconfig.Config{HTTPProxy: "http://component.example:3128"},
			wantSource:    resourcesynccontroller.ResourceLocation{},
			wantSyncCalls: 1,
		},
		{
			name:            "clearing the source fails",
			proxy:           nil,
			syncRejected:    true,
			wantSource:      resourcesynccontroller.ResourceLocation{},
			wantReason:      "FailedResourceSyncUpdate",
			wantErrContains: "registering authentication proxy CA sync",
			wantSyncCalls:   1,
		},
		{
			name:          "copied CA is already present",
			proxy:         withCustomTrust,
			sourceCA:      sourceCAValid,
			copiedCA:      copiedCAPresent,
			wantSource:    configSource,
			wantSyncCalls: 1,
		},
		{
			name:            "waiting for the copied CA",
			proxy:           withCustomTrust,
			sourceCA:        sourceCAValid,
			copiedCA:        copiedCAPending,
			wantSource:      configSource,
			wantReason:      "AwaitingLocalConfigMapSync",
			wantErrContains: "waiting for authentication proxy CA ConfigMap",
			wantSyncError:   true,
			wantSyncCalls:   1,
		},
		{
			name:            "registering the sync rule fails",
			proxy:           withCustomTrust,
			sourceCA:        sourceCAValid,
			syncRejected:    true,
			wantSource:      configSource,
			wantReason:      "FailedResourceSyncUpdate",
			wantErrContains: "registering authentication proxy CA sync",
			wantSyncCalls:   1,
		},
		{
			name:            "reading the copied CA fails",
			proxy:           withCustomTrust,
			sourceCA:        sourceCAValid,
			copiedCA:        copiedCALookupFails,
			wantSource:      configSource,
			wantReason:      "FailedGetLocalConfigMap",
			wantErrContains: caLookupMessage,
			wantSyncCalls:   1,
		},
		{
			name:            "source CA ConfigMap is not found",
			proxy:           withCustomTrust,
			sourceCA:        sourceCAPending,
			wantReason:      "SourceNotFound",
			wantErrContains: "not found",
		},
		{
			name:            "source CA ConfigMap lookup fails",
			proxy:           withCustomTrust,
			sourceCA:        sourceCALookupFails,
			wantReason:      "FailedGetSource",
			wantErrContains: sourceLookupMessage,
		},
		{
			name:            "source CA ConfigMap is missing the required key",
			proxy:           withCustomTrust,
			sourceCA:        sourceCAMissingKey,
			wantReason:      "InvalidSource",
			wantErrContains: "missing required key",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syncer := &recordingResourceSyncer{}
			if tc.syncRejected {
				syncer.err = errors.New("sync rule rejected")
			}
			co := &consoleOperator{
				resourceSyncer:          syncer,
				targetNSConfigMapLister: destinationLister(t, tc.copiedCA, caLookupMessage),
				configNSConfigMapLister: sourceLister(t, tc.sourceCA, "proxy-ca", sourceLookupMessage),
			}

			reason, err := co.SyncAuthProxyTrustedCAConfigMap(tc.proxy)

			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if tc.wantErrContains == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrContains)) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErrContains)
			}
			if consoleerrors.IsSyncError(err) != tc.wantSyncError {
				t.Errorf("IsSyncError = %t, want %t (err: %v)", consoleerrors.IsSyncError(err), tc.wantSyncError, err)
			}
			if syncer.calls != tc.wantSyncCalls {
				t.Errorf("SyncConfigMap called %d times, want %d", syncer.calls, tc.wantSyncCalls)
			}
			if tc.wantSyncCalls > 0 && (syncer.destination != wantDestination || syncer.source != tc.wantSource) {
				t.Errorf("registered sync rule = {dest: %v, source: %v}, want {dest: %v, source: %v}", syncer.destination, syncer.source, wantDestination, tc.wantSource)
			}
		})
	}
}

// destinationLister returns a ConfigMap lister for the console namespace seeded
// to match the requested state of the copied proxy CA.
func destinationLister(t *testing.T, state copiedCAState, lookupErrorMessage string) corelisters.ConfigMapLister {
	t.Helper()
	if state == copiedCALookupFails {
		return &failingConfigMapLister{err: errors.New(lookupErrorMessage)}
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if state == copiedCAPresent {
		if err := indexer.Add(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}}); err != nil {
			t.Fatal(err)
		}
	}
	return corelisters.NewConfigMapLister(indexer)
}

// sourceLister returns a ConfigMap lister for the openshift-config namespace
// seeded to match the requested state of the administrator-supplied proxy CA.
func sourceLister(t *testing.T, state sourceCAState, name, lookupErrorMessage string) corelisters.ConfigMapLister {
	t.Helper()
	if state == sourceCALookupFails {
		return &failingConfigMapLister{err: errors.New(lookupErrorMessage)}
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if state == sourceCAValid || state == sourceCAMissingKey {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: api.OpenShiftConfigNamespace, Name: name}}
		if state == sourceCAValid {
			cm.Data = map[string]string{api.AuthProxyCAFileName: "test-ca-data"}
		}
		if err := indexer.Add(cm); err != nil {
			t.Fatal(err)
		}
	}
	return corelisters.NewConfigMapLister(indexer)
}

// TestConsoleTeardownRemovesTrustedCAConfigMap verifies that tearing down the console (Removed
// management state) deletes the copied proxy CA and clears its sync source.
func TestConsoleTeardownRemovesTrustedCAConfigMap(t *testing.T) {
	op := &operatorv1.Console{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}}
	op.Spec.ManagementState = operatorv1.Removed
	kubeClient := fake.NewClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: api.AuthProxyCAConfigMapName, Namespace: api.TargetNamespace}})
	syncer := &recordingResourceSyncer{}
	co := &consoleOperator{
		resourceSyncer:   syncer,
		configMapClient:  kubeClient.CoreV1(),
		secretsClient:    kubeClient.CoreV1(),
		deploymentClient: kubeClient.AppsV1(),
		operatorClient:   v1helpers.NewFakeOperatorClient(&op.Spec.OperatorSpec, &operatorv1.OperatorStatus{}, nil),
	}
	recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))

	if err := co.handleSync(t.Context(), factory.NewSyncContext("test", recorder), configSet{Operator: op}); err != nil {
		t.Fatal(err)
	}
	if _, err := kubeClient.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.AuthProxyCAConfigMapName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("proxy CA was not deleted: %v", err)
	}
	if syncer.calls != 1 || syncer.source != (resourcesynccontroller.ResourceLocation{}) {
		t.Fatalf("sync source was not cleared: %#v", syncer)
	}
}

type recordingResourceSyncer struct {
	destination, source resourcesynccontroller.ResourceLocation
	calls               int
	err                 error
}

func (s *recordingResourceSyncer) SyncConfigMap(destination, source resourcesynccontroller.ResourceLocation) error {
	s.destination, s.source = destination, source
	s.calls++
	return s.err
}

func (s *recordingResourceSyncer) SyncSecret(_, _ resourcesynccontroller.ResourceLocation) error {
	return nil
}

// failingConfigMapLister returns err from every destination ConfigMap lookup.
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
