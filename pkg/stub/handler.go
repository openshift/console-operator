package stub

import (
	"context"
	"github.com/sirupsen/logrus"

	// TODO: use when swapping up to client from Handler
	// "k8s.io/api/apps/v1"
	// "k8s.io/client-go/rest"
	// v12 "k8s.io/client-go/kubernetes/typed/apps/v1"
	// k8sv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// TODO: use when swapping up to client from Handler
	// "github.com/openshift/origin/pkg/route/apis/route"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/operator"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
)

// TODO: use when swapping up to client from Handler
// TODO: pass in the namespace
// TODO: create speific clients
// TODO: rename "default" objects that are currently being
//   created manually then mutated via sdk.Get(obj), sdk.Update(obj), etc
//   the clients in the rest of the todo's do NOT mutate the obj passed in
//   instead they will return a new obj as would be a little more
func NewHandler() sdk.Handler {
	return &handler{}

	// TODO: use when swapping up to client from Handler
	//conf, err := rest.InClusterConfig()
	//
	//if err != nil {
	//	panic(err)
	//}
	//
	// k8sv1Client := k8sv1.NewForConfigOrDie(conf)
	// v12Client := v12.NewForConfigOrDie(conf)
	// routeClient
	//
	//return &handler{
	//	// configMapClient
	//	deploymentsClient: v12Client.Deployments(namespace),
	//	eventsClient: k8sv1Client.Events(namespace),
	//	// routesClient:
	//	secretsClient: k8sv1Client.Secrets(namespace),
	//	servicesClient: k8sv1Client.Services(namespace),
	//}
}

type handler struct {
	// TODO: use when swapping up to client from Handler
	// configMapClient
	// deploymentsClient v12.DeploymentInterface
	// eventsClient k8sv1.EventInterface
	// routesClient
	// secretsClient k8sv1.SecretInterface
	// servicesClient k8sv1.ServiceInterface
}

func (h *handler) Handle(_ context.Context, event sdk.Event) error {
	cr, err := getCR()
	if isDeleted(err) {
		logrus.Info("console has been deleted.")
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
	return operator.ReconcileConsole(cr)
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