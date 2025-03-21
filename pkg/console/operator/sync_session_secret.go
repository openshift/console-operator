package operator

import (
	"context"

	// kube
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
)

func (co *consoleOperator) syncSessionSecret(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	recorder events.Recorder,
) (*corev1.Secret, error) {

	sessionSecret, err := co.secretsLister.Secrets(api.TargetNamespace).Get(api.SessionSecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	var required *corev1.Secret
	if sessionSecret == nil {
		required = secretsub.DefaultSessionSecret(operatorConfig)
	} else {
		required = sessionSecret.DeepCopy()
		changed := secretsub.ResetSessionSecretKeysIfNeeded(required)
		if !changed {
			return required, nil
		}
	}

	secret, _, err := resourceapply.ApplySecret(ctx, co.secretsClient, recorder, required)
	return secret, err
}
