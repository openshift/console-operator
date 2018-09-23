package stub

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"

	"github.com/openshift/console-operator/pkg/operator"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/sets"

	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

func NewFilteredHandler(handler sdk.Handler) sdk.Handler {
	// all secondary resources have this name
	validNames := sets.NewString(
		operator.OpenShiftConsoleName,
		operator.OpenShiftConsoleShortName,
		operator.ConsoleConfigMapName,
		operator.OAuthClientName,
		operator.ConsoleConfigMapName,
		operator.ConsoleServingCertName,
		operator.ConsoleOauthConfigName,
	)
	// Casting an anonymous function as a HandlerFunc to
	// filter out irrelevant events before they hit our handler
	return HandlerFunc(func(context context.Context, event sdk.Event) error {
		switch event.Object.(type) {
		case
			*routev1.Route,
			*corev1.Service,
			*corev1.ConfigMap,
			*corev1.Secret,
			*appsv1.Deployment,
			*oauthv1.OAuthClient:
			// secondary resource types we care about,
			// check the name below before deciding to handle
		case *v1alpha1.Console:
			// cr.yaml metadata.name should not change form "openshift-console"
			// that said, if we decide it is configurable, we probabaly need to
			// handle the Console all the time.
			// return handler.Handle(context, event)
		default:
			// this should never happen, it means that something
			// else has been deployed into our namespace and is
			// interfering with our environment.
			return errors.New("unknown type")
		}

		// now, if the resource is the correct type,
		// make sure it is also one of our named resources
		object, err := meta.Accessor(event.Object)
		if err != nil {
			// this should never happen.
			// event.Object is a runtime.Object, it
			// is certainly going to work with meta.Accessor
			return err
		}
		if !validNames.Has(object.GetName()) {
			// we don't care about these.
			// someone deployed something unexpected
			// in our namespace
			logrus.Warnf("ignoring resource %v named %v", object.GetSelfLink(), object.GetName())
			return nil
		}

		// if we make it this far, we should handle the event
		return handler.Handle(context, event)
	})
}
