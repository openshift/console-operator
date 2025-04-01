package operator

import (
	"context"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

func (co *consoleOperator) SyncDeployment(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	cm *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	oauthServingCertConfigMap *corev1.ConfigMap,
	authServerCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	sec *corev1.Secret,
	sessionSecret *corev1.Secret,
	proxyConfig *configv1.Proxy,
	infrastructureConfig *configv1.Infrastructure,
	recorder events.Recorder,
) (consoleDeployment *appsv1.Deployment, changed bool, reason string, err error) {
	updatedOperatorConfig := operatorConfig.DeepCopy()
	requiredDeployment := deploymentsub.DefaultDeployment(
		operatorConfig,
		cm,
		serviceCAConfigMap,
		oauthServingCertConfigMap,
		authServerCAConfigMap,
		trustedCAConfigMap,
		sec,
		sessionSecret,
		proxyConfig,
		infrastructureConfig,
	)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment, ctx)

	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
		ctx,
		co.deploymentClient,
		recorder,
		requiredDeployment,
		resourcemerge.ExpectedDeploymentGeneration(requiredDeployment, updatedOperatorConfig.Status.Generations),
	)

	if applyDepErr != nil {
		return nil, false, "FailedApply", applyDepErr
	}
	return deployment, deploymentChanged, "", nil
}
