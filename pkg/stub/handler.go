package stub

import (
	"context"
	"fmt"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	// appsv1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	"github.com/openshift/console-operator/pkg/console"

	routev1 "github.com/openshift/api/route/v1"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
	// NOTE: none of the example operators seem to fill this.
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	fmt.Printf("ConsoleHandler.Handle() %o", event)
	// Event: Created, Updated, Deleted
	// but Created + Updated are kind of the same :)
	// TODO:
	// - If Delete, don't deploy again :)
	//   - Delete should be cleanup
	// - If Paused, skip reconcile loop & do nothing. let the user fiddle manually
	// - If Changed?  reconcile()
	//   - make sure object state is honored
	// fmt.Printf("Got a: %o", event.Object.GetObjectKind())
	switch o := event.Object.(type) {
	case *v1alpha1.Console:
		// Vault version has some vault.Reconcile function:
		// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/stub/handler.go#L22
		// this is probably a good idea!
		err := console.Reconcile(o)
		if err != nil && !errors.IsAlreadyExists(err) {
			logrus.Errorf("failed to reconcile origin web console : %v", err)
			return err
		}
	case *routev1.Route:
		fmt.Printf("Got a route", o)
	}

	return nil
}
