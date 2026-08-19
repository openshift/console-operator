package starter

import (
	"context"
	"reflect"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	v1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
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
