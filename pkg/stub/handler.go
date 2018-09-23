package stub

import (
	"context"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/console-operator/pkg/operator"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
)

func NewHandler() sdk.Handler {
	// andy bootstrapping? if so, we can do it here.
	return &handler{}
}

type handler struct {
}

func (h *handler) Handle(_ context.Context, event sdk.Event) error {
	cr, err := getCR()
	if isDeleted(err) {
		logrus.Info("console has been deleted.")
		cleanup()
		return nil
	}
	// some kind of non-404 error?
	if err != nil {
		return err
	}

	if isPaused(cr) {
		logrus.Info("console has been paused.")
		return nil
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

func isPaused(console *v1alpha1.Console) bool {
	if console.Spec.Paused {
		return true
	}
	return false
}

func isDeleted(err error) bool {
	//if err != nil {
	//	if !errors.IsNotFound(err) {
	//		return true
	//	}
	//	return false
	//}
	return errors.IsNotFound(err)
}

func cleanup() {
	logrus.Info("console has been deleted.")
	// Garbage collector cleans up most via ownersRefs
	operator.DeleteOauthClient()
}
