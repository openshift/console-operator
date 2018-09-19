package stub

import (
	"context"
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/operator"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {

	if event.Deleted {
		// Garbage collector will take care of most secondary resources
		// thanks to ownersRefs
		operator.DeleteOauthClient()
		return nil
	}

	// Event: Created, Updated, Deleted
	//   Create & Update are similar
	// TODO:
	// - If Paused
	//   - Skip reconcile loop & do nothing
	// - If Change
	//   - make sure object state is honored
	switch o := event.Object.(type) {
	case *routev1.Route:
		logrus.Info("HANDLE: *routev1.Route >>>>>>>>")
		cr, err := h.getConsole()
		if err != nil {
			return err
		}
		operator.UpdateOauthClient(cr, o)
		operator.UpdateConsoleConfigMap(cr, o)
		cr.UpdateHost(o)
	// TODO: watch & handle each of the secondary resources created.
	case *corev1.Service:
		logrus.Info("HANDLE: *corev1.Service >>>>>>>>")
	case *corev1.ConfigMap:
		logrus.Info("HANDLE: *corev1.ConfigMap >>>>>>>>")
	case *corev1.Secret:
		logrus.Info("HANDLE: *corev1.Secret >>>>>>>>")
	case *appsv1.Deployment:
		logrus.Info("HANDLE: *appsv1.Deployment >>>>>>>>")
	case *oauthv1.OAuthClient:
		// NOTE: this should never fire, it does not exist in our namespace
		logrus.Info("HANDLE: *oauthv1.OAuthClient >>>>>>>>")
	case *v1alpha1.Console:
		logrus.Info("HANDLE: *v1alpha1.Console >>>>>>>>")
		changed := o.SetDefaults()
		logrus.Info("Defaults updated:", changed)
		operator.DeployConsole(o)
	}

	return nil
}

func (h *Handler) getConsole() (*v1alpha1.Console, error) {
	namespace, _ := k8sutil.GetWatchNamespace()
	cr := &v1alpha1.Console{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Console",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-console", // openshiftConsoleName in utils, circular dep?
			Namespace: namespace,
		},
	}
	err := sdk.Get(cr)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get console custom resource: %s", err)
		}
		return nil, nil
	}
	return cr, nil
}
