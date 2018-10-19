package stub

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/operator"
)

func NewHandler() sdk.Handler {
	// andy bootstrapping? if so, we can do it here.
	return &handler{}
}

type handler struct {
}

func (h *handler) Handle(_ context.Context, event sdk.Event) error {
	cr, err := getCR()

	// if the CR is deleted, the operator will immediately create a
	// new one.
	if errors.IsNotFound(err) {
		fmt.Printf("Console does not exist, creating default console \n")
		if _, err := operator.ApplyConsole(); !errors.IsAlreadyExists(err) {
			logrus.Infof("Console deleted, attempting to recreate %v", err)
		}
		return nil
	}
	// some kind of non-404 error?
	if err != nil {
		return err
	}

	fmt.Printf("management state is - %v \n", cr.Spec.ManagementState)

	switch cr.Spec.ManagementState {
	case operatorv1alpha1.Managed:
		// handled below
	case operatorv1alpha1.Unmanaged:
		return nil
	case operatorv1alpha1.Removed:
		return operator.DeleteAllResources(cr)
	default:
		// TODO should update status
		return fmt.Errorf("unknown management state: %v \n", cr.Spec.ManagementState)
	}

	logrus.Info("reconciling console...")
	// create all of the resources if they do not exist
	// then ensure they are in the correct state
	// enforcing shared secrets, route.Host, etc
	operator.ReconcileConsole(cr)

	return err
}

func getCR() (*v1alpha1.Console, error) {
	namespace, _ := k8sutil.GetWatchNamespace()
	cr := &v1alpha1.Console{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Console",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operator.OpenShiftConsoleName,
			Namespace: namespace,
		},
	}
	err := sdk.Get(cr)
	return cr, err
}
