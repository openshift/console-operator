package starter

import (
	"context"
	"reflect"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	fakeconfigclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	v1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestGetResourceSyncerInformersCacheSync(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	operatorClient := v1helpers.NewFakeOperatorClient(
		&operatorv1.OperatorSpec{ManagementState: operatorv1.Managed},
		&operatorv1.OperatorStatus{},
		nil,
	)
	recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
	controllerCtx := &controllercmd.ControllerContext{
		EventRecorder: recorder,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resourceSyncerInformers, _ := getResourceSyncer(controllerCtx, kubeClient, operatorClient)
	resourceSyncerInformers.Start(ctx.Done())

	// Verify that ConfigMap and Secret informers for all resource syncer namespaces
	// can sync their caches. The ResourceSyncController registers both ConfigMap and
	// Secret informers for each namespace; if any fail to sync, the controller will
	// never start and configmap syncing (e.g. oauth-serving-cert) won't happen.
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (done bool, err error) {
		for ns := range resourceSyncerInformers.Namespaces() {
			if len(ns) == 0 {
				continue
			}
			inf := resourceSyncerInformers.InformersFor(ns)
			if !inf.Core().V1().ConfigMaps().Informer().HasSynced() {
				return false, nil
			}
			if !inf.Core().V1().Secrets().Informer().HasSynced() {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("resource syncer informers failed to sync caches: %v", err)
	}
}

func TestDeduplicateObjectReferences(t *testing.T) {
	tests := []struct {
		name     string
		input    []configv1.ObjectReference
		expected []configv1.ObjectReference
	}{
		{
			name:     "no duplicates",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g2", Resource: "r2", Name: "n2"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g2", Resource: "r2", Name: "n2"}},
		},
		{
			name:     "with duplicates",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g1", Resource: "r1", Name: "n1"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}},
		},
		{
			name:     "different namespace not duplicate",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns2"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns2"}},
		},
		{
			name:     "all fields equal",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateObjectReferences(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("deduplicateObjectReferences() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPollAndCallOnIngressEnabled(t *testing.T) {
	infraConfig := &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: api.ConfigResourceName},
		Status: configv1.InfrastructureStatus{
			ControlPlaneTopology: configv1.ExternalTopologyMode,
		},
	}
	clusterVersionIngressDisabled := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: api.VersionResourceName},
		Status: configv1.ClusterVersionStatus{
			Capabilities: configv1.ClusterVersionCapabilitiesStatus{
				EnabledCapabilities: []configv1.ClusterVersionCapability{
					configv1.ClusterVersionCapabilityOpenShiftSamples,
				},
			},
		},
	}

	t.Run("does not trigger when ingress stays disabled", func(t *testing.T) {
		configClient := fakeconfigclient.NewClientset(infraConfig, clusterVersionIngressDisabled)

		triggered := make(chan struct{})
		pollAndCallOnIngressEnabled(t.Context(), configClient, 50*time.Millisecond, func() {
			close(triggered)
		})

		select {
		case <-triggered:
			t.Fatal("callback should not have been called when ingress is disabled")
		case <-time.After(500 * time.Millisecond):
		}
	})

	t.Run("triggers when ingress becomes enabled", func(t *testing.T) {
		configClient := fakeconfigclient.NewClientset(infraConfig, clusterVersionIngressDisabled)

		triggered := make(chan struct{})
		pollAndCallOnIngressEnabled(t.Context(), configClient, 50*time.Millisecond, func() {
			close(triggered)
		})

		// Wait for at least one poll cycle, then enable ingress
		time.Sleep(100 * time.Millisecond)
		cv, err := configClient.ConfigV1().ClusterVersions().Get(context.Background(), api.VersionResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get ClusterVersion: %v", err)
		}
		cv.Status.Capabilities.EnabledCapabilities = append(cv.Status.Capabilities.EnabledCapabilities, configv1.ClusterVersionCapabilityIngress)
		if _, err := configClient.ConfigV1().ClusterVersions().UpdateStatus(context.Background(), cv, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to update ClusterVersion status: %v", err)
		}

		select {
		case <-triggered:
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for callback after ingress was enabled")
		}
	})
}
