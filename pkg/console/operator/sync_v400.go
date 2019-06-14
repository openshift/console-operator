package operator

import (
	"errors"
	"fmt"
	"os"

	// kube
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/util"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	configmapsub "github.com/openshift/console-operator/pkg/console/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/deployment"
	// operator
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	oauthsub "github.com/openshift/console-operator/pkg/console/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/route"
	secretsub "github.com/openshift/console-operator/pkg/console/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/service"
)

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func sync_v400(co *consoleOperator, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure) error {
	klog.V(4).Infoln("running sync loop 4.0.0")
	recorder := co.recorder

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	rt, rtChanged, rtErr := SyncRoute(co, operatorConfig)
	if rtErr != nil {
		msg := fmt.Sprintf("%v: %s", "route", rtErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return rtErr
	}
	toUpdate = toUpdate || rtChanged

	svc, svcChanged, svcErr := SyncService(co, recorder, operatorConfig)
	if svcErr != nil {
		msg := fmt.Sprintf("%q: %v", "service", svcErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return svcErr
	}
	toUpdate = toUpdate || svcChanged

	cm, cmChanged, cmErr := SyncConfigMap(co, recorder, operatorConfig, consoleConfig, infrastructureConfig, rt)
	if cmErr != nil {
		msg := fmt.Sprintf("%q: %v", "configmap", cmErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return cmErr
	}
	toUpdate = toUpdate || cmChanged

	serviceCAConfigMap, serviceCAConfigMapChanged, serviceCAConfigMapErr := SyncServiceCAConfigMap(co, operatorConfig)
	if serviceCAConfigMapErr != nil {
		msg := fmt.Sprintf("%q: %v", "serviceCAconfigmap", serviceCAConfigMapErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return serviceCAConfigMapErr
	}
	toUpdate = toUpdate || serviceCAConfigMapChanged

	sec, secChanged, secErr := SyncSecret(co, recorder, operatorConfig)
	if secErr != nil {
		msg := fmt.Sprintf("%q: %v", "secret", secErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return secErr
	}
	toUpdate = toUpdate || secChanged

	oauthClient, oauthChanged, oauthErr := SyncOAuthClient(co, operatorConfig, sec, rt)
	if oauthErr != nil {
		msg := fmt.Sprintf("%q: %v", "oauth", oauthErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return oauthErr
	}
	toUpdate = toUpdate || oauthChanged

	actualDeployment, depChanged, depErr := SyncDeployment(co, recorder, operatorConfig, cm, serviceCAConfigMap, sec, rt)
	if depErr != nil {
		msg := fmt.Sprintf("%q: %v", "deployment", depErr)
		klog.V(4).Infof("incomplete sync: %v", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return depErr
	}
	toUpdate = toUpdate || depChanged

	resourcemerge.SetDeploymentGeneration(&operatorConfig.Status.Generations, actualDeployment)
	operatorConfig.Status.ObservedGeneration = operatorConfig.ObjectMeta.Generation

	klog.V(4).Infoln("-----------------------")
	klog.V(4).Infof("sync loop 4.0.0 resources updated: %v", toUpdate)
	klog.V(4).Infoln("-----------------------")

	// if we made it this far, the operator is not failing
	// but we will handle the state of the operand below
	co.ConditionResourceSyncSuccess(operatorConfig)
	// the operand is in a transitional state if any of the above resources changed
	// or if we have not settled on the desired number of replicas or deployment is not uptodate.
	if toUpdate {
		co.ConditionResourceSyncProgressing(operatorConfig, "Changes made during sync updates, additional sync expected.")
	} else {
		version := os.Getenv("RELEASE_VERSION")
		if !deploymentsub.IsAvailableAndUpdated(actualDeployment) {
			co.ConditionResourceSyncProgressing(operatorConfig, fmt.Sprintf("Working toward version %s", version))
		} else {
			if co.versionGetter.GetVersions()["operator"] != version {
				co.versionGetter.SetVersion("operator", version)
			}
			co.ConditionResourceSyncNotProgressing(operatorConfig)
		}
	}

	// the operand is available if all resources are:
	// - present
	// - if we have at least one ready replica
	// - route is admitted
	// available is currently defined as "met the users intent"
	if !deploymentsub.IsReady(actualDeployment) {
		msg := fmt.Sprintf("%v pods available for console deployment", actualDeployment.Status.ReadyReplicas)
		klog.V(4).Infoln(msg)
		co.ConditionDeploymentNotAvailable(operatorConfig, msg)
	} else if !routesub.IsAdmitted(rt) {
		klog.V(4).Infoln("console route is not admitted")
		co.SetStatusCondition(
			operatorConfig,
			operatorv1.OperatorStatusTypeAvailable,
			operatorv1.ConditionFalse,
			"RouteNotAdmitted",
			"console route is not admitted",
		)
	} else if actualDeployment.Status.Replicas == actualDeployment.Status.ReadyReplicas && actualDeployment.Status.Replicas == actualDeployment.Status.UpdatedReplicas {
		co.ConditionDeploymentAvailable(operatorConfig, fmt.Sprintf("%v replicas ready at version %s", actualDeployment.Status.ReadyReplicas, os.Getenv("RELEASE_VERSION")))
	} else {
		co.ConditionDeploymentAvailable(operatorConfig, fmt.Sprintf("%v replicas ready", actualDeployment.Status.ReadyReplicas))
	}

	// if we survive the gauntlet, we need to update the console config with the
	// public hostname so that the world can know the console is ready to roll
	klog.V(4).Infoln("sync_v400: updating console status")
	consoleURL := getConsoleURL(rt)
	if consoleURL == "" {
		err := customerrors.NewSyncError("waiting on route host")
		klog.Errorf("%q: %v", "route", err)
		return err
	}

	if _, err := SyncConsoleConfig(co, consoleConfig, consoleURL); err != nil {
		klog.Errorf("could not update console config status: %v", err)
		return err
	}

	if _, _, err := SyncConsolePublicConfig(co, recorder, consoleURL); err != nil {
		klog.Errorf("could not update public console config status: %v", err)
		return err
	}

	defer func() {
		klog.V(4).Infof("sync loop 4.0.0 complete")
		if svcChanged {
			klog.V(4).Infof("\t service changed: %v", svc.GetResourceVersion())
		}
		if rtChanged {
			klog.V(4).Infof("\t route changed: %v", rt.GetResourceVersion())
		}
		if cmChanged {
			klog.V(4).Infof("\t configmap changed: %v", cm.GetResourceVersion())
		}
		if serviceCAConfigMapChanged {
			klog.V(4).Infof("\t service-ca configmap changed: %v", serviceCAConfigMap.GetResourceVersion())
		}
		if secChanged {
			klog.V(4).Infof("\t secret changed: %v", sec.GetResourceVersion())
		}
		if oauthChanged {
			klog.V(4).Infof("\t oauth changed: %v", oauthClient.GetResourceVersion())
		}
		if depChanged {
			klog.V(4).Infof("\t deployment changed: %v", actualDeployment.GetResourceVersion())
		}
	}()

	return nil
}

func getConsoleURL(route *routev1.Route) string {
	host := routesub.GetCanonicalHost(route)
	if host == "" {
		return ""
	}
	return util.HTTPS(host)
}

func SyncConsoleConfig(co *consoleOperator, consoleConfig *configv1.Console, consoleURL string) (*configv1.Console, error) {
	if consoleConfig.Status.ConsoleURL != consoleURL {
		klog.V(4).Infof("updating console.config.openshift.io with url: %v", consoleURL)
		consoleConfig.Status.ConsoleURL = consoleURL
	}
	return co.consoleConfigClient.UpdateStatus(consoleConfig)
}

func SyncConsolePublicConfig(co *consoleOperator, recorder events.Recorder, consoleURL string) (*corev1.ConfigMap, bool, error) {
	requiredConfigMap := configmapsub.DefaultPublicConfig(consoleURL)
	return resourceapply.ApplyConfigMap(co.configMapClient, recorder, requiredConfigMap)
}

func SyncDeployment(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, cm *corev1.ConfigMap, serviceCAConfigMap *corev1.ConfigMap, sec *corev1.Secret, rt *routev1.Route) (*appsv1.Deployment, bool, error) {
	requiredDeployment := deploymentsub.DefaultDeployment(operatorConfig, cm, serviceCAConfigMap, sec, rt)
	expectedGeneration := getDeploymentGeneration(co)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment)

	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
		co.deploymentClient,
		recorder,
		requiredDeployment,
		expectedGeneration,
		// redeploy on operatorConfig.spec changes
		genChanged,
	)

	if applyDepErr != nil {
		klog.Errorf("%q: %v", "deployment", applyDepErr)
		return nil, false, applyDepErr
	}
	return deployment, deploymentChanged, nil
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func SyncOAuthClient(co *consoleOperator, operatorConfig *operatorv1.Console, sec *corev1.Secret, rt *routev1.Route) (*oauthv1.OAuthClient, bool, error) {
	host := routesub.GetCanonicalHost(rt)
	if host == "" {
		customErr := customerrors.NewSyncError("waiting on route host")
		klog.Errorf("%q: %v", "oauth", customErr)
		return nil, false, customErr
	}
	oauthClient, err := co.oauthClient.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("%q: %v", "oauth", err)
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, errors.New("oauth client for console does not exist and cannot be created")
	}
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, host, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.CustomApplyOAuth(co.oauthClient, oauthClient)
	if oauthErr != nil {
		klog.Errorf("%q: %v", "oauth", oauthErr)
		return nil, false, oauthErr
	}
	return oauthClient, oauthChanged, nil
}

func SyncSecret(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console) (*corev1.Secret, bool, error) {
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		return resourceapply.ApplySecret(co.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
	}
	// any error should be returned & kill the sync loop
	if err != nil {
		klog.Errorf("%q: %v", "secret", err)
		return nil, false, err
	}
	return secret, false, nil
}

// apply configmap (needs route)
// by the time we get to the configmap, we can assume the route exits & is configured properly
// therefore no additional error handling is needed here.
func SyncConfigMap(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure, rt *routev1.Route) (*corev1.ConfigMap, bool, error) {
	managedConfig, mcErr := co.configMapClient.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if mcErr != nil && !apierrors.IsNotFound(mcErr) {
		klog.Errorf("managed config error: %v", mcErr)
		return nil, false, mcErr
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, managedConfig, infrastructureConfig, rt)
	if err != nil {
		return nil, false, err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		klog.Errorf("%q: %v", "configmap", cmErr)
		return nil, false, cmErr
	}
	if cmChanged {
		klog.V(4).Infoln("new console config yaml:")
		klog.V(4).Infof("%s", cm.Data)
	}
	return cm, cmChanged, cmErr
}

// apply service-ca configmap
func SyncServiceCAConfigMap(co *consoleOperator, operatorConfig *operatorv1.Console) (*corev1.ConfigMap, bool, error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(required)
		if err == nil {
			klog.V(4).Infoln("service-ca configmap created")
		} else {
			klog.Errorf("%q: %v", "service-ca configmap", err)
		}
		return actual, true, err
	}
	if err != nil {
		klog.Errorf("%q: %v", "service-ca configmap", err)
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("service-ca configmap exists and is in the correct state")
		return existing, false, nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(existing)
	if err == nil {
		klog.V(4).Infoln("service-ca configmap updated")
	} else {
		klog.Errorf("%q: %v", "service-ca configmap", err)
	}
	return actual, true, err
}

// apply service
// there is nothing special about our service, so no additional error handling is needed here.
func SyncService(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console) (*corev1.Service, bool, error) {
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, recorder, servicesub.DefaultService(operatorConfig))
	if svcErr != nil {
		klog.Errorf("%q: %v", "service", svcErr)
		return nil, false, svcErr
	}
	return svc, svcChanged, svcErr
}

// apply route
// - be sure to test that we don't trigger an infinite loop by stomping on the
//   default host name set by the server, or any other values. The ApplyRoute()
//   logic will have to be sound.
// - update to ApplyRoute() once the logic is settled
func SyncRoute(co *consoleOperator, operatorConfig *operatorv1.Console) (*routev1.Route, bool, error) {
	// ensure we have a route. any error returned is a non-404 error
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(operatorConfig))
	if rtErr != nil {
		klog.Errorf("%q: %v", "route", rtErr)
		return nil, false, rtErr
	}

	// we will not proceed until the route is valid. this eliminates complexity with the
	// configmap, secret & oauth client as they can be certain they have a host if we pass this point.
	host := routesub.GetCanonicalHost(rt)
	if host == "" {
		customErr := customerrors.NewSyncError("waiting on route host")
		klog.Errorf("%q: %v", "route", customErr)
		return nil, false, customErr
	}

	if validatedRoute, changed := routesub.Validate(rt); changed {
		// if validation changed the route, issue an update
		if _, err := co.routeClient.Routes(api.TargetNamespace).Update(validatedRoute); err != nil {
			// error is unexpected, this is a real error
			klog.Errorf("%q: %v", "route", err)
			return nil, false, err
		}
		// abort sync, route changed, let it settle & retry
		customErr := customerrors.NewSyncError("route is invalid, correcting route state")
		klog.Error(customErr)
		return nil, true, customErr
	}
	// only return the route if it is valid with a host
	return rt, rtIsNew, rtErr
}

func getDeploymentGeneration(co *consoleOperator) int64 {
	deployment, err := co.deploymentClient.Deployments(api.TargetNamespace).Get(deploymentsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}
