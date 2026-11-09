package operator

import (
	"bytes"
	"flag"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	clocktesting "k8s.io/utils/clock/testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/api"
	consoleerrors "github.com/openshift/console-operator/pkg/console/errors"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/telemetry"
	"github.com/openshift/console-operator/pkg/proxyconfig"
)

// TestSyncAwaitsAuthProxyCABeforePublishingConfigMap verifies that sync_v400
// does not publish (or otherwise touch) the console-config ConfigMap or the
// Deployment while the auth proxy's trusted CA has not yet been copied into the
// target namespace -- i.e. SyncAuthProxyTrustedCAConfigMap gates SyncConfigMap,
// preserving whatever Console configuration was already in place.
func TestSyncAwaitsAuthProxyCABeforePublishingConfigMap(t *testing.T) {
	co, kubeClient, configs := newAuthProxyReconcileFixture(t, copiedCAPending)
	recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))

	err := co.handleSync(t.Context(), factory.NewSyncContext("test", recorder), configs)
	if err == nil {
		t.Fatal("expected an error while awaiting the auth proxy CA copy, got nil")
	}
	if !consoleerrors.IsSyncError(err) {
		t.Errorf("expected a retriable sync error, got: %v", err)
	}

	if _, getErr := kubeClient.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{}); !apierrors.IsNotFound(getErr) {
		t.Errorf("expected console-config ConfigMap to not be published yet, error returned: %v", getErr)
	}
	if _, getErr := kubeClient.AppsV1().Deployments(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleDeploymentName, metav1.GetOptions{}); !apierrors.IsNotFound(getErr) {
		t.Errorf("expected console Deployment to not be created yet, error returned: %v", getErr)
	}
}

// Adding the CA is staged: the CA copy is there, the Deployment is rolled out with
// the mount, and only then the CA path is written to the Console configuration.
//
// The test starts from a clean cluster, advances the process one step at a time
// and asserts the state after each step. "Mounted" always means that the
// Deployment template has the volume, not that every Pod has it. What the Pods
// have is what the Deployment status says, which the test sets by hand.
//
// The CA is configured from the start and its copy in openshift-console is already
// there (the fixture serves it from a static lister). The wait for a missing copy
// is covered by TestSyncAwaitsAuthProxyCABeforePublishingConfigMap.
func TestSyncStagesAuthProxyCARollout(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)

	// Step 0: clean cluster. There is no Deployment and no Console configuration.
	if _, err := client.AppsV1().Deployments(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleDeploymentName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("step 0: expected no Deployment, got: %v", err)
	}
	if _, err := client.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("step 0: expected no Console configuration, got: %v", err)
	}

	// Step 1: first sync. The Deployment is created with the CA volume, and the
	// configuration is created without the CA path. Nothing is running yet, but
	// every Pod the Deployment creates later will have the mount.
	r.sync()
	assertAuthProxyCA(t, client, "step 1: first sync", true, false)

	// Step 2: the Deployment controller is working on the template. Each of these
	// states means that some Pod may lack the mount, so the path must stay out of
	// the configuration, however many syncs happen meanwhile.
	for _, rollout := range []struct {
		name           string
		updated, total int32
	}{
		{"nothing observed yet", 0, 0},
		{"old Pods are still counted", 0, 1},
		{"a surge Pod is still present", 1, 2},
	} {
		r.setRollout(rollout.updated, rollout.total)
		r.sync()
		assertAuthProxyCA(t, client, "step 2: rollout in progress ("+rollout.name+")", true, false)
	}

	// Step 3: every Pod comes from the template with the mount. The path is
	// published now. No replica is ready in the fixture, which must not matter:
	// readiness says nothing about the mount and would block on unrelated Pod problems.
	r.completeRollout()
	r.sync()
	assertAuthProxyCA(t, client, "step 3: rollout complete", true, true)

	// Step 4: the path is sticky. Another rollout is in progress, so the gate of
	// step 3 does not hold, but the path is already published, and an unrelated
	// configuration repair must not take it back.
	r.setRollout(1, 2)
	co.authProxyResolver = stubProxyResolver{config: &proxyconfig.Config{TrustedCAName: "user-ca-bundle", HTTPSProxy: "https://repaired-proxy.example:3128"}}
	r.sync()
	assertAuthProxyCA(t, client, "step 4: unrelated rollout and repair", true, true)
	cm, err := client.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cm.Data["console-config.yaml"], "repaired-proxy.example") {
		t.Fatal("step 4: configuration repair was not published")
	}
}

// Removing the CA reverses the order of adding it: the CA path leaves the Console
// configuration first, then the Deployment drops the mount, and the CA copy goes
// last, once no Pod can mount it anymore.
//
// The test starts from a console with the CA fully rolled out, advances the process
// one step at a time and asserts the state after each step. "Mounted" means that the
// Deployment template has the volume, not that every Pod has it. What the Pods have
// is what the Deployment status says, which the test sets by hand. "Copy registered"
// means that the resource syncer is still asked to keep the CA copy in openshift-console.
func TestSyncStagesAuthProxyCARemoval(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)
	syncer := co.resourceSyncer.(*recordingResourceSyncer)
	assertStage := func(step string, wantMounted, wantReferenced, wantCopyRegistered bool) {
		t.Helper()
		assertAuthProxyCA(t, client, step, wantMounted, wantReferenced)
		if got := syncer.source.Name != ""; got != wantCopyRegistered {
			t.Fatalf("%s: CA copy registered = %v, want %v", step, got, wantCopyRegistered)
		}
	}

	// Step 0: the CA is in use. The Deployment mounts it and every Pod has it, so
	// the configuration references the path, and the CA copy is kept in sync.
	r.publishCA()
	assertStage("step 0: CA in use", true, true, true)

	// Step 1: the CA is removed from the proxy configuration. The very same sync
	// rewrites the configuration without the path. Mount and copy stay: the Pods
	// that are running now were started with the old configuration and may still
	// be reading the file, and so may be the ones that restart before the new
	// configuration reaches them.
	co.authProxyResolver = stubProxyResolver{config: &proxyconfig.Config{HTTPSProxy: "https://proxy.example:3128"}}
	r.sync()
	assertStage("step 1: CA removed from the proxy configuration", true, false, true)

	// Step 2: the published configuration no longer references the file, so nothing
	// that starts from now on needs the mount. The Deployment template drops it.
	// The copy stays: the old Pods have not been replaced yet and still mount it.
	// Nothing changes in between: the configuration written by the previous sync is
	// what this one reads, and in a cluster, those writes are what requeue the controller.
	r.sync()
	assertStage("step 2: mount dropped", false, false, true)

	// Step 3: the Deployment controller is replacing the Pods. As long as old Pods
	// are around, the copy has to stay, however many syncs happen meanwhile.
	r.setRollout(0, 1)
	r.sync()
	assertStage("step 3: rollout in progress", false, false, true)

	// Step 4: every Pod comes from the template without the mount. No Pod can mount
	// the copy anymore, so it is not kept in sync any longer.
	r.completeRollout()
	r.sync()
	assertStage("step 4: rollout complete", false, false, false)
}

// If the CA copy is already gone there is nothing to keep mounted.
func TestSyncDoesNotRetainMissingAuthProxyCA(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)
	r.publishCA()
	if err := client.CoreV1().ConfigMaps(api.TargetNamespace).Delete(t.Context(), api.AuthProxyCAConfigMapName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	refreshAuthProxyConfigMapLister(t, co, client)
	co.authProxyResolver = stubProxyResolver{}
	r.sync()
	if authProxyMounted(t, client) {
		t.Fatal("mount kept for a CA ConfigMap that no longer exists")
	}
}

// The informers lag behind the API server and behind each other. The tests below
// take a snapshot of what the informers show at one point, let the process move
// on, and then run a sync in which one informer still shows the snapshot. Each
// test first asserts that the snapshot shows the misleading state it is about.

// The previous sync published the path, which started a rollout. The Deployment
// informer already shows that rollout, the ConfigMap informer still shows the
// configuration without the path. The path must not be withdrawn: that would
// cost two needless rollouts.
func TestSyncKeepsAuthProxyCAPathWhenConfigMapInformerLags(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)

	// Step 1: bring the CA to the point right before its path is published. The
	// first sync creates the Deployment with the CA mounted and the configuration
	// without the path. Then every Pod comes from that template.
	r.sync()
	r.completeRollout()

	// Remember what the informers show now: the configuration without the path.
	// The ConfigMap informer is going to be stuck here in step 3.
	beforePublishing := r.snapshot()
	if beforePublishing.referencesCA(t) {
		t.Fatal("precondition: the snapshot should show the configuration without the path")
	}

	// Step 2: the rollout is complete, so this sync publishes the path. That changes
	// the configuration, whose hash is in the Deployment template, so the same sync
	// also updates the Deployment and starts a new rollout. That rollout has nothing
	// to do with the mount, which every Pod already has.
	r.sync()
	assertAuthProxyCA(t, client, "path published", true, true)
	// The Deployment informer has seen that update, but the Deployment controller
	// has not observed it yet, so the Deployment does not count as rolled out,
	// although the mount is there.
	if d := r.snapshot().deployment(t); d.Status.ObservedGeneration >= d.Generation {
		t.Fatal("precondition: the Deployment informer should show the rollout started by publishing the path")
	}

	// Step 3: a sync in which the Deployment informer has seen the write of step 2,
	// but the ConfigMap informer has not. Going by the informers alone, the path is
	// not published and the Deployment is not rolled out, so the path would be
	// withdrawn, which starts a rollout, and once that completes, published again,
	// which starts another one. The operator remembers the configuration it applied
	// in step 2, though, so it knows the path is published and keeps it.
	r.syncWithStaleConfigMaps(beforePublishing)
	assertAuthProxyCA(t, client, "sync with a lagging ConfigMap informer", true, true)
}

// The CA is re-added while the Deployment is rolling out without the mount, but
// the Deployment informer still shows the template that had it, fully rolled out.
// The path must not be published while Pods without the mount are being started.
func TestSyncDoesNotPublishAuthProxyCAPathWhenDeploymentInformerLags(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)
	withCA := co.authProxyResolver
	withoutCA := stubProxyResolver{config: &proxyconfig.Config{HTTPProxy: "http://proxy.example:3128"}}

	// Step 1: the CA is fully in use. publishCA ends with the sync that publishes
	// the path, which starts a rollout because the configuration hash changed.
	// Completing that rollout leaves every Pod with the mount and the path.
	r.publishCA()
	r.completeRollout()

	// Remember what the informers show now: the Deployment rolled out with the mount.
	// The Deployment informer is going to be stuck here in step 3.
	mountedEverywhere := r.snapshot()
	if !deploymentsub.RolledOutWithVolume(mountedEverywhere.deployment(t), api.AuthProxyCAConfigMapName) {
		t.Fatal("precondition: the snapshot should show the Deployment rolled out with the mount")
	}

	// Step 2: the CA is removed. The first sync drops the path from the configuration,
	// the second one drops the mount from the Deployment template. The Deployment
	// controller is now replacing the Pods with ones that do not have the mount.
	co.authProxyResolver = withoutCA
	r.sync()
	r.sync()
	assertAuthProxyCA(t, client, "CA removed and unmounted", false, false)

	// Step 3: the CA is re-added, in a sync in which the ConfigMap informer has seen
	// the writes of step 2, but the Deployment informer has not. Going by the
	// informers alone, the Deployment is rolled out with the mount, so the path
	// would be published right away, while Pods without the mount, started by
	// step 2, would read it and fail to find the file. The operator remembers that
	// it updated the Deployment in step 2, though, so it ignores the older Deployment
	// the informer shows. The mount is added back, but the path waits for its rollout.
	co.authProxyResolver = withCA
	r.syncWithStaleDeployments(mountedEverywhere)
	assertAuthProxyCA(t, client, "CA re-added with a lagging Deployment informer", true, false)
}

// The CA is added, so the Deployment controller is rolling out a template with the
// mount, but the Deployment informer still shows the earlier template, rolled out
// without it. Removing the CA now must not remove the copy the new Pods are about to mount.
func TestSyncKeepsAuthProxyCACopyWhenDeploymentInformerLags(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)
	syncer := co.resourceSyncer.(*recordingResourceSyncer)
	withCA := co.authProxyResolver
	withoutCA := stubProxyResolver{config: &proxyconfig.Config{HTTPProxy: "http://proxy.example:3128"}}

	// Step 1: no CA is configured. The first sync creates the Deployment without the
	// mount, then every Pod comes from that template.
	co.authProxyResolver = withoutCA
	r.sync()
	r.completeRollout()

	// Remember what the informers show now: the Deployment rolled out without the mount.
	// The Deployment informer is going to be stuck here in step 3.
	mountedNowhere := r.snapshot()
	if !deploymentsub.RolledOutWithoutVolume(mountedNowhere.deployment(t), api.AuthProxyCAConfigMapName) {
		t.Fatal("precondition: the snapshot should show the Deployment rolled out without the mount")
	}

	// Step 2: the CA is added. The sync keeps the CA copy in sync and adds the mount
	// to the Deployment template. The path waits for the rollout. The Deployment
	// controller is now starting Pods that mount the copy.
	co.authProxyResolver = withCA
	r.sync()
	assertAuthProxyCA(t, client, "CA added", true, false)

	// Step 3: the CA is removed again, in a sync in which the Deployment informer
	// has not seen the write of step 2. The path was never published, so all that
	// is left is to drop the mount and the copy. Going by the informers alone, the
	// Deployment is rolled out without the mount, so the copy would be removed right
	// away, while the Pods started by step 2 still need it: the volume is not optional,
	// so they would not start. The operator remembers that it updated the Deployment
	// in step 2, though, so it ignores the older Deployment the informer shows. The
	// mount is dropped, but the copy stays until that rollout completes.
	co.authProxyResolver = withoutCA
	r.syncWithStaleDeployments(mountedNowhere)
	if syncer.source.Name == "" {
		t.Fatal("CA copy unregistered while the Deployment is rolling out with the mount")
	}
}

// Degraded set for a missing CA source is cleared by the first sync after the CA
// is removed, although that sync skips the copy sync that would otherwise clear it.
func TestSyncClearsAuthProxyTrustedCASyncDegradedWhenCARemoved(t *testing.T) {
	co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
	r := newAuthProxyReconciler(t, co, client, configs)
	r.publishCA()

	co.authProxyResolver = stubProxyResolver{config: &proxyconfig.Config{TrustedCAName: "missing-ca-bundle", HTTPProxy: "http://proxy.example:3128"}}
	if err := co.handleSync(t.Context(), factory.NewSyncContext("test", r.recorder), configs); err == nil {
		t.Fatal("expected an error for a missing CA source ConfigMap")
	}
	if !authProxyCADegraded(t, co) {
		t.Fatal("precondition: AuthProxyTrustedCASyncDegraded should be True")
	}

	co.authProxyResolver = stubProxyResolver{config: &proxyconfig.Config{HTTPProxy: "http://proxy.example:3128"}}
	r.sync()
	assertAuthProxyCA(t, client, "CA removed", true, false)
	if authProxyCADegraded(t, co) {
		t.Fatal("AuthProxyTrustedCASyncDegraded still True after the CA was removed")
	}
}

func TestSyncConfigMapDoesNotLogProxyCredentials(t *testing.T) {
	for _, verbosity := range []string{"2", "4"} {
		t.Run("verbosity="+verbosity, func(t *testing.T) {
			co, client, configs := newAuthProxyReconcileFixture(t, copiedCAPresent)
			co.authProxyResolver = stubProxyResolver{}
			recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
			sync := func() error { return co.handleSync(t.Context(), factory.NewSyncContext("test", recorder), configs) }
			if err := sync(); err != nil {
				t.Fatal(err)
			}

			refreshAuthProxyConfigMapLister(t, co, client)

			state := klog.CaptureState()
			defer state.Restore()

			flags := flag.NewFlagSet("logging", flag.ContinueOnError)
			klog.InitFlags(flags)
			if err := flags.Set("v", verbosity); err != nil {
				t.Fatal(err)
			}
			klog.LogToStderr(false)
			var logs bytes.Buffer
			klog.SetOutput(&logs)

			co.authProxyResolver = stubProxyResolver{config: &proxyconfig.Config{HTTPProxy: "http://review-user:review-password@review-tail@proxy.example:3128"}}
			if err := sync(); err != nil {
				t.Fatal(err)
			}

			klog.Flush()

			cm, err := client.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(cm.Data["console-config.yaml"], "review-password") {
				t.Fatal("credentials were not persisted to Console configuration")
			}

			for _, credential := range []string{"review-user", "review-password", "review-tail"} {
				if strings.Contains(logs.String(), credential) {
					t.Fatalf("proxy credential %q leaked in logs", credential)
				}
			}
		})
	}
}

// newAuthProxyReconcileFixture builds a consoleOperator wired with the minimal
// set of fakes/listers needed to drive a full Managed-state sync_v400 reconcile
// for an OIDC console with an authProxy.trustedCA configured, parameterized by
// the readiness state of the copied proxy CA ConfigMap.
func newAuthProxyReconcileFixture(t *testing.T, caState copiedCAState) (*consoleOperator, *fake.Clientset, configSet) {
	t.Helper()

	const consoleURL = "https://console-openshift-console.apps.example.com"

	op := &operatorv1.Console{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}}
	op.Spec.ManagementState = operatorv1.Managed
	op.Spec.Ingress.ConsoleURL = consoleURL

	consoleConfig := &configv1.Console{
		ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName},
		Status:     configv1.ConsoleStatus{ConsoleURL: consoleURL},
	}

	authnConfig := &configv1.Authentication{
		ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName},
		Spec: configv1.AuthenticationSpec{
			Type: configv1.AuthenticationTypeOIDC,
			OIDCProviders: []configv1.OIDCProvider{{
				Name: "test-oidc",
				Issuer: configv1.TokenIssuer{
					URL:       "https://idp.example.com",
					Audiences: []configv1.TokenAudience{"console"},
				},
				OIDCClients: []configv1.OIDCClientConfig{{
					ComponentNamespace: api.TargetNamespace,
					ComponentName:      api.OpenShiftConsoleName,
					ClientID:           "console-client",
				}},
			}},
		},
	}

	featureGate := &configv1.FeatureGate{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}}
	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: api.VersionResourceName},
		Spec:       configv1.ClusterVersionSpec{ClusterID: "test-cluster-id"},
	}
	telemetryConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: api.OpenShiftConsoleOperatorNamespace, Name: telemetry.TelemetryConfigMapName},
	}
	oauthClientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: api.TargetNamespace, Name: secretsub.Stub().Name},
	}
	consoleServingCertSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: api.TargetNamespace, Name: api.ConsoleServingCertName},
	}

	mustAdd := func(indexer cache.Indexer, obj any) {
		if err := indexer.Add(obj); err != nil {
			t.Fatal(err)
		}
	}

	authnIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	mustAdd(authnIndexer, authnConfig)

	featureGateIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	mustAdd(featureGateIndexer, featureGate)

	clusterVersionIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	mustAdd(clusterVersionIndexer, clusterVersion)

	operatorNSConfigMapIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	mustAdd(operatorNSConfigMapIndexer, telemetryConfigMap)

	secretsIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	mustAdd(secretsIndexer, oauthClientSecret)
	mustAdd(secretsIndexer, consoleServingCertSecret)

	// The copy is served by both the client and the lister, so that refreshing the
	// lister from the client keeps it, and deleting it from the client removes it.
	var objects []runtime.Object
	if caState == copiedCAPresent {
		objects = append(objects, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: api.TargetNamespace, Name: api.AuthProxyCAConfigMapName}})
	}
	kubeClient := fake.NewClientset(objects...)

	co := &consoleOperator{
		operatorClient:             v1helpers.NewFakeOperatorClient(&op.Spec.OperatorSpec, &operatorv1.OperatorStatus{}, nil),
		authnConfigLister:          configlistersv1.NewAuthenticationLister(authnIndexer),
		featureGateLister:          configlistersv1.NewFeatureGateLister(featureGateIndexer),
		clusterVersionLister:       configlistersv1.NewClusterVersionLister(clusterVersionIndexer),
		operatorNSConfigMapLister:  corev1listers.NewConfigMapLister(operatorNSConfigMapIndexer),
		managedNSConfigMapLister:   corev1listers.NewConfigMapLister(newIndexer(cache.MetaNamespaceKeyFunc)),
		targetNSConfigMapLister:    destinationLister(t, caState, "auth proxy CA lookup failed"),
		configNSConfigMapLister:    sourceLister(t, sourceCAValid, "user-ca-bundle", "auth proxy trustedCA lookup failed"),
		nodeLister:                 corev1listers.NewNodeLister(newIndexer(cache.MetaNamespaceKeyFunc)),
		monitoringDeploymentLister: appsv1listers.NewDeploymentLister(newIndexer(cache.MetaNamespaceKeyFunc)),
		configNSSecretLister:       corev1listers.NewSecretLister(newIndexer(cache.MetaNamespaceKeyFunc)),
		secretsLister:              corev1listers.NewSecretLister(secretsIndexer),
		secretsClient:              kubeClient.CoreV1(),
		configMapClient:            kubeClient.CoreV1(),
		deploymentClient:           kubeClient.AppsV1(),
		deploymentLister:           appsv1listers.NewDeploymentLister(newIndexer(cache.MetaNamespaceKeyFunc)),
		resourceSyncer:             &recordingResourceSyncer{},
		authProxyResolver: stubProxyResolver{config: &proxyconfig.Config{
			TrustedCAName: "user-ca-bundle",
			HTTPProxy:     "http://proxy.example:3128",
		}},
		versionGetter: status.NewVersionGetter(),
	}
	co.trackables.isOLMDisabled = true
	// Pre-seed the telemetry org/account cache so GetTelemetryConfiguration takes
	// the cached-values path instead of making a real OCM subscription request.
	co.trackables.organizationID = "test-org"
	co.trackables.accountMail = "test@example.com"

	configs := configSet{
		Console:        consoleConfig,
		Operator:       op,
		Infrastructure: &configv1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}},
		Proxy:          &configv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}},
		OAuth:          &configv1.OAuth{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}},
		Ingress:        &configv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName}},
	}

	return co, kubeClient, configs
}

// stubProxyResolver resolves a fixed component proxy configuration, bypassing
// the real GatedProxyResolver/ConsoleProxyResolver feature-gate plumbing.
type stubProxyResolver struct {
	config *proxyconfig.Config
}

func (s stubProxyResolver) ResolveProxy(*operatorv1.Console) (*proxyconfig.Config, error) {
	return s.config, nil
}

// authProxyReconciler drives repeated sync_v400 reconciles, modelling what the
// API server and the informers do between them.
type authProxyReconciler struct {
	t        *testing.T
	co       *consoleOperator
	client   *fake.Clientset
	configs  configSet
	recorder events.Recorder
}

func newAuthProxyReconciler(t *testing.T, co *consoleOperator, client *fake.Clientset, configs configSet) *authProxyReconciler {
	// Like the real API server, bump the generation on spec updates, so that a
	// template change is not seen as rolled out until the controller observes it.
	client.PrependReactor("update", "deployments", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "" {
			dep := action.(clientgotesting.UpdateAction).GetObject().(*appsv1.Deployment)
			dep.Generation++
		}
		return false, nil, nil
	})
	return &authProxyReconciler{
		t: t, co: co, client: client, configs: configs,
		recorder: events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now())),
	}
}

// sync runs a reconcile that must succeed, then lets the informers catch up.
func (r *authProxyReconciler) sync() {
	r.t.Helper()
	if err := r.co.handleSync(r.t.Context(), factory.NewSyncContext("test", r.recorder), r.configs); err != nil {
		r.t.Fatal(err)
	}
	refreshAuthProxyListers(r.t, r.co, r.client)
}

// syncWithStaleConfigMaps runs a reconcile that must succeed while the ConfigMap
// informer still shows the snapshot, then lets the informers catch up.
func (r *authProxyReconciler) syncWithStaleConfigMaps(stale informerSnapshot) {
	r.t.Helper()
	r.co.targetNSConfigMapLister = stale.configMaps
	r.sync()
}

// syncWithStaleDeployments runs a reconcile that must succeed while the Deployment
// informer still shows the snapshot, then lets the informers catch up.
func (r *authProxyReconciler) syncWithStaleDeployments(stale informerSnapshot) {
	r.t.Helper()
	r.co.deploymentLister = stale.deployments
	r.sync()
}

// informerSnapshot is what the informers showed at some point.
type informerSnapshot struct {
	configMaps  corev1listers.ConfigMapLister
	deployments appsv1listers.DeploymentLister
}

// snapshot returns a deep copy of what the informers show now. Being a copy, it
// keeps showing that however the informers are refreshed afterwards.
func (r *authProxyReconciler) snapshot() informerSnapshot {
	r.t.Helper()
	configMaps, err := r.co.targetNSConfigMapLister.List(labels.Everything())
	if err != nil {
		r.t.Fatal(err)
	}
	configMapIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	for _, cm := range configMaps {
		if err := configMapIndexer.Add(cm.DeepCopy()); err != nil {
			r.t.Fatal(err)
		}
	}
	deployments, err := r.co.deploymentLister.List(labels.Everything())
	if err != nil {
		r.t.Fatal(err)
	}
	deploymentIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	for _, d := range deployments {
		if err := deploymentIndexer.Add(d.DeepCopy()); err != nil {
			r.t.Fatal(err)
		}
	}
	return informerSnapshot{
		configMaps:  corev1listers.NewConfigMapLister(configMapIndexer),
		deployments: appsv1listers.NewDeploymentLister(deploymentIndexer),
	}
}

// referencesCA returns true if the Console configuration in the snapshot references the CA path.
func (s informerSnapshot) referencesCA(t *testing.T) bool {
	t.Helper()
	cm, err := s.configMaps.ConfigMaps(api.TargetNamespace).Get(api.OpenShiftConsoleConfigMapName)
	if err != nil {
		t.Fatal(err)
	}
	referenced, err := authProxyCAReferenced(cm)
	if err != nil {
		t.Fatal(err)
	}
	return referenced
}

// deployment returns the console Deployment in the snapshot.
func (s informerSnapshot) deployment(t *testing.T) *appsv1.Deployment {
	t.Helper()
	d, err := s.deployments.Deployments(api.TargetNamespace).Get(api.OpenShiftConsoleDeploymentName)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// setRollout records the Deployment status and lets the informers catch up.
func (r *authProxyReconciler) setRollout(updated, total int32) {
	r.t.Helper()
	setRollout(r.t, r.client, updated, total)
	refreshAuthProxyListers(r.t, r.co, r.client)
}

// completeRollout marks the current Deployment template as fully rolled out.
func (r *authProxyReconciler) completeRollout() {
	r.t.Helper()
	r.setRollout(1, 1)
}

// publishCA takes the fixture's configured CA all the way to a published path.
func (r *authProxyReconciler) publishCA() {
	r.t.Helper()
	r.sync()
	r.completeRollout()
	r.sync()
	if !authProxyReferenced(r.t, r.client) {
		r.t.Fatal("precondition: CA path should be published")
	}
}

// assertAuthProxyCA checks whether the Deployment template mounts the proxy CA and
// whether the published Console configuration references its path.
func assertAuthProxyCA(t *testing.T, client *fake.Clientset, step string, wantMounted, wantReferenced bool) {
	t.Helper()
	if got := authProxyMounted(t, client); got != wantMounted {
		t.Fatalf("%s: Deployment mounts the CA = %v, want %v", step, got, wantMounted)
	}
	if got := authProxyReferenced(t, client); got != wantReferenced {
		t.Fatalf("%s: configuration references the CA = %v, want %v", step, got, wantReferenced)
	}
}

func authProxyReferenced(t *testing.T, client *fake.Clientset) bool {
	t.Helper()
	cm, err := client.CoreV1().ConfigMaps(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	referenced, err := authProxyCAReferenced(cm)
	if err != nil {
		t.Fatal(err)
	}
	return referenced
}

func authProxyMounted(t *testing.T, client *fake.Clientset) bool {
	t.Helper()
	dep, err := client.AppsV1().Deployments(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	volume, mount := false, false
	for _, v := range dep.Spec.Template.Spec.Volumes {
		volume = volume || v.Name == api.AuthProxyCAConfigMapName
	}
	for _, m := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
		mount = mount || m.Name == api.AuthProxyCAConfigMapName
	}
	return volume && mount
}

// setRollout records the Deployment status as seen by the Deployment controller.
// Fake clients do not maintain it; the fixture runs a single replica. Readiness is deliberately left at zero.
func setRollout(t *testing.T, client *fake.Clientset, updated, total int32) {
	t.Helper()
	dep, err := client.AppsV1().Deployments(api.TargetNamespace).Get(t.Context(), api.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dep.Status.ObservedGeneration = dep.Generation
	dep.Status.UpdatedReplicas = updated
	dep.Status.Replicas = total
	if _, err := client.AppsV1().Deployments(api.TargetNamespace).UpdateStatus(t.Context(), dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// Model the informers observing the objects written by the previous reconcile.
func refreshAuthProxyListers(t *testing.T, co *consoleOperator, client *fake.Clientset) {
	t.Helper()
	refreshAuthProxyConfigMapLister(t, co, client)
	deployments, err := client.AppsV1().Deployments(api.TargetNamespace).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	indexer := newIndexer(cache.MetaNamespaceKeyFunc)
	for i := range deployments.Items {
		if err := indexer.Add(&deployments.Items[i]); err != nil {
			t.Fatal(err)
		}
	}
	co.deploymentLister = appsv1listers.NewDeploymentLister(indexer)
}

// Model the informer observing the ConfigMaps in openshift-console.
func refreshAuthProxyConfigMapLister(t *testing.T, co *consoleOperator, client *fake.Clientset) {
	t.Helper()
	configMaps, err := client.CoreV1().ConfigMaps(api.TargetNamespace).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	indexer := newIndexer(cache.MetaNamespaceKeyFunc)
	for i := range configMaps.Items {
		if err := indexer.Add(&configMaps.Items[i]); err != nil {
			t.Fatal(err)
		}
	}
	co.targetNSConfigMapLister = corev1listers.NewConfigMapLister(indexer)
}

func authProxyCADegraded(t *testing.T, co *consoleOperator) bool {
	t.Helper()
	_, st, _, err := co.operatorClient.GetOperatorState()
	if err != nil {
		t.Fatal(err)
	}
	cond := v1helpers.FindOperatorCondition(st.Conditions, "AuthProxyTrustedCASyncDegraded")
	return cond != nil && cond.Status == operatorv1.ConditionTrue
}
