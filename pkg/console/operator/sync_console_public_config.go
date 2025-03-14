package operator

import (
	"context"

	// kube
	corev1 "k8s.io/api/core/v1"

	// openshift
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"

	// operator
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
)

func (co *consoleOperator) SyncConsolePublicConfig(ctx context.Context, consoleURL string, recorder events.Recorder) (*corev1.ConfigMap, bool, error) {
	requiredConfigMap := configmapsub.DefaultPublicConfig(consoleURL)
	return resourceapply.ApplyConfigMap(ctx, co.configMapClient, recorder, requiredConfigMap)
}
