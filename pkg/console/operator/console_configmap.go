package operator

import (
	"context"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
)

// applyConsoleConfigMap applies Console configuration without logging its data.
func (c *consoleOperator) applyConsoleConfigMap(ctx context.Context, recorder events.Recorder, required *corev1.ConfigMap) (*corev1.ConfigMap, bool, error) {
	ctx = klog.NewContext(ctx, logr.Discard())
	return resourceapply.ApplyConfigMap(ctx, c.configMapClient, recorder, required)
}
