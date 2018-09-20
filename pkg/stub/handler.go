package stub

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"

	"k8s.io/apimachinery/pkg/runtime"

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

func (h *handler) Handle(_ context.Context, _ sdk.Event) error {

	// checking a few things to test if deleted now, see isDeleted
	// if event.Deleted {
	//	// Garbage collector cleans up most via ownersRefs
	//	operator.DeleteOauthClient()
	//	return nil
	// }

	cr, err := getCR()
	if err != nil {
		return err
	}
	if isDeleted(cr, err) {
		logrus.Info("console has been deleted.")
		// Garbage collector cleans up most via ownersRefs
		operator.DeleteOauthClient()
		return nil
	}

	if isPaused(cr) {
		logrus.Info("console has been paused.")
		return nil
	}

	// operator.Reconcile(cr)
	// at this point we need to do the following:
	//   create deployment if not exists
	//   create service if not exists
	//   create route if not exists
	//   create configmap if not exists
	//   create oauthclient if not exists
	// 		which will look something like this:
	//        sdk.Get(the-client)
	//        if !exists
	//          sdk.Get(the-route)
	//          addRouteHostIfWeGotIt(the-client)
	//          sdk.Create(the-client)
	//        else
	//          sdk.Get(the-route)
	//          addRouteHostIfWeGotIt(the-client)
	//          sdk.Update(the-client)
	//   create oauthclient-secret if not exists
	// but also
	//   sync random secret between oauthclient & oauthclient-secret
	//   sync route.host between route, oauthclient.redirectURIs & configmap.baseAddress
	operator.CreateConsoleDeployment(cr)
	logrus.Info("Time to do real things now!  Nothing is deleted, nothing is paused....")
	return nil
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
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get console custom resource: %s", err)
		}
		return nil, nil
	}
	return cr, nil
}

func isDeleted(object runtime.Object, err error) bool {
	logrus.Info("isDeleted() ?")
	logrus.Printf("isDeleted() ?")
	logrus.Printf("isDeleted() ? %#v", object)
	logrus.Printf("isDeleted() ? %s", object)
	logrus.Printf("isDeleted() ?")
	logrus.Printf("isDeleted() ?")

	if object == nil {
		return true
	}

	// 404 deleted
	if errors.IsNotFound(err) {
		return true
	}
	// this is an error on the get request
	if err != nil {
		return false
	}
	// in process of being deleted
	obj, err := meta.Accessor(object)

	logrus.Printf("What is this nonsense %#v", obj)

	if obj == nil {
		logrus.Print("I dunno, its nil er something....")
		return true
	}

	//if err != nil || obj == nil {
	//	return false
	//}

	if obj.GetDeletionTimestamp() != nil {
		return false
	}
	return false
}

func isPaused(console *v1alpha1.Console) bool {
	if console.Spec.Paused {
		return true
	}
	return false
}
