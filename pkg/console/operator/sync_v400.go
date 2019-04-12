package operator

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/openshift/console-operator/pkg/console/subresource/util"

	"github.com/openshift/console-operator/pkg/api"

	// 3rd party
	"github.com/sirupsen/logrus"
	// kube
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/subresource/service"
)

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func sync_v400(co *consoleOperator, originalOperatorConfig *operatorv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure) (*operatorv1.Console, *configv1.Console, bool, error) {
	operatorConfig := originalOperatorConfig.DeepCopy()
	logrus.Println("running sync loop 4.0.0")
	recorder := co.recorder

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	rt, rtChanged, rtErr := SyncRoute(co, operatorConfig)
	if rtErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%v: %s\n", "route", rtErr)))
		return operatorConfig, consoleConfig, toUpdate, rtErr
	}
	toUpdate = toUpdate || rtChanged

	_, svcChanged, svcErr := SyncService(co, recorder, operatorConfig)
	if svcErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "service", svcErr)))
		return operatorConfig, consoleConfig, toUpdate, svcErr
	}
	toUpdate = toUpdate || svcChanged

	cm, cmChanged, cmErr := SyncConfigMap(co, recorder, operatorConfig, consoleConfig, infrastructureConfig, rt)
	if cmErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "configmap", cmErr)))
		return operatorConfig, consoleConfig, toUpdate, cmErr
	}
	toUpdate = toUpdate || cmChanged

	serviceCAConfigMap, serviceCAConfigMapChanged, serviceCAConfigMapErr := SyncServiceCAConfigMap(co, operatorConfig)
	if serviceCAConfigMapErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "serviceCAconfigmap", serviceCAConfigMapErr)))
		return operatorConfig, consoleConfig, toUpdate, serviceCAConfigMapErr
	}
	toUpdate = toUpdate || serviceCAConfigMapChanged

	sec, secChanged, secErr := SyncSecret(co, recorder, operatorConfig)
	if secErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "secret", secErr)))
		return operatorConfig, consoleConfig, toUpdate, secErr
	}
	toUpdate = toUpdate || secChanged

	_, oauthChanged, oauthErr := SyncOAuthClient(co, operatorConfig, sec, rt)
	if oauthErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "oauth", oauthErr)))
		return operatorConfig, consoleConfig, toUpdate, oauthErr
	}
	toUpdate = toUpdate || oauthChanged

	actualDeployment, depChanged, depErr := SyncDeployment(co, recorder, operatorConfig, cm, serviceCAConfigMap, sec)
	if depErr != nil {
		co.SyncStatus(co.ConditionResourceSyncFailure(operatorConfig, fmt.Sprintf("%q: %v\n", "deployment", depErr)))
		return operatorConfig, consoleConfig, toUpdate, depErr
	}
	toUpdate = toUpdate || depChanged

	resourcemerge.SetDeploymentGeneration(&operatorConfig.Status.Generations, actualDeployment)
	operatorConfig.Status.ObservedGeneration = operatorConfig.ObjectMeta.Generation

	logrus.Println("-----------------------")
	logrus.Printf("sync loop 4.0.0 resources updated: %v \n", toUpdate)
	logrus.Println("-----------------------")

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
			co.ConditionResourceSyncProgressing(operatorConfig, fmt.Sprintf("Moving to version %s", strings.Split(version, "-")[0]))
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
		co.ConditionDeploymentNotAvailable(operatorConfig)
	} else if !routesub.IsAdmitted(rt) {
		co.SetStatusCondition(
			operatorConfig,
			operatorv1.OperatorStatusTypeAvailable,
			operatorv1.ConditionFalse,
			"RouteNotAdmitted",
			"Console route is not admitted",
		)
	} else {
		co.ConditionDeploymentAvailable(operatorConfig)
	}

	// finally write out the set of conditions currently set if anything has changed
	// to avoid a hot loop
	if !reflect.DeepEqual(operatorConfig, originalOperatorConfig) {
		co.SyncStatus(operatorConfig)
	}

	// if we survive the gauntlet, we need to update the console config with the
	// public hostname so that the world can know the console is ready to roll
	logrus.Println("sync_v400: updating console status")
	if updatedConfig, err := SyncConsoleConfig(co, consoleConfig, rt); err != nil {
		logrus.Errorf("could not update console config status: %v \n", err)
		return operatorConfig, updatedConfig, toUpdate, err
	}

	defer func() {
		logrus.Printf("sync loop 4.0.0 complete:")
		logrus.Printf("\t service changed: %v", svcChanged)
		logrus.Printf("\t route changed: %v", rtChanged)
		logrus.Printf("\t configMap changed: %v", cmChanged)
		logrus.Printf("\t secret changed: %v", secChanged)
		logrus.Printf("\t oauth changed: %v", oauthChanged)
		logrus.Printf("\t deployment changed: %v", depChanged)
	}()

	return operatorConfig, consoleConfig, toUpdate, nil
}

func SyncConsoleConfig(co *consoleOperator, consoleConfig *configv1.Console, route *routev1.Route) (*configv1.Console, error) {
	host, err := routesub.GetCanonicalHost(route)
	if err != nil {
		return nil, err
	}
	logrus.Printf("updating console.config.openshift.io with hostname: %v \n", host)
	consoleConfig.Status.ConsoleURL = util.HTTPS(host)
	return co.consoleConfigClient.UpdateStatus(consoleConfig)
}

func SyncDeployment(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, cm *corev1.ConfigMap, serviceCAConfigMap *corev1.ConfigMap, sec *corev1.Secret) (*appsv1.Deployment, bool, error) {
	logrus.Printf("validating console deployment...")
	requiredDeployment := deploymentsub.DefaultDeployment(operatorConfig, cm, serviceCAConfigMap, sec)
	expectedGeneration := getDeploymentGeneration(co)
	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
		co.deploymentClient,
		recorder,
		requiredDeployment,
		expectedGeneration,
		// redeploy on operatorConfig.spec changes
		operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration,
	)

	if applyDepErr != nil {
		logrus.Errorf("%q: %v \n", "deployment", applyDepErr)
		return nil, false, applyDepErr
	}
	logrus.Println("deployment exists and is in the correct state")
	return deployment, deploymentChanged, nil
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func SyncOAuthClient(co *consoleOperator, operatorConfig *operatorv1.Console, sec *corev1.Secret, rt *routev1.Route) (*oauthv1.OAuthClient, bool, error) {
	logrus.Printf("validating oauthclient...")
	host, err := routesub.GetCanonicalHost(rt)
	if err != nil {
		return nil, false, err
	}
	oauthClient, err := co.oauthClient.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("%q: %v \n", "oauth", err)
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, errors.New("oauth client for console does not exist.")
	}
	// this should take care of all of our syncronization
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, host, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.ApplyOAuth(co.oauthClient, oauthClient)
	if oauthErr != nil {
		logrus.Errorf("%q: %v \n", "oauth", oauthErr)
		return nil, false, oauthErr
	}
	logrus.Println("oauthclient exists and is in the correct state")
	return oauthClient, oauthChanged, nil
}

func SyncSecret(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console) (*corev1.Secret, bool, error) {
	logrus.Printf("validating oauth secret...")
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		return resourceapply.ApplySecret(co.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
	}
	// any error should be returned & kill the sync loop
	if err != nil {
		logrus.Errorf("%q: %v \n", "secret", err)
		return nil, false, err
	}
	logrus.Println("secret exists and is in the correct state")
	return secret, false, nil
}

// apply configmap (needs route)
// by the time we get to the configmap, we can assume the route exits & is configured properly
// therefore no additional error handling is needed here.
func SyncConfigMap(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure, rt *routev1.Route) (*corev1.ConfigMap, bool, error) {
	logrus.Printf("validating console configmap...")

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, infrastructureConfig, rt)
	if err != nil {
		return nil, false, err
	}

	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		logrus.Errorf("%q: %v \n", "configmap", cmErr)
		return nil, false, cmErr
	}

	logrus.Println("configmap exists and is in the correct state")
	return cm, cmChanged, cmErr
}

// apply service-ca configmap
func SyncServiceCAConfigMap(co *consoleOperator, operatorConfig *operatorv1.Console) (*corev1.ConfigMap, bool, error) {
	logrus.Printf("validating service-ca configmap...")
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(required)
		if err == nil {
			logrus.Println("service-ca configmap created")
		} else {
			logrus.Errorf("%q: %v \n", "service-ca configmap", err)
		}
		return actual, true, err
	}
	if err != nil {
		logrus.Errorf("%q: %v \n", "service-ca configmap", err)
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		logrus.Println("service-ca configmap exists and is in the correct state")
		return existing, false, nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(existing)
	if err == nil {
		logrus.Println("service-ca configmap updated")
	} else {
		logrus.Errorf("%q: %v \n", "service-ca configmap", err)
	}
	return actual, true, err
}

// apply service
// there is nothing special about our service, so no additional error handling is needed here.
func SyncService(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console) (*corev1.Service, bool, error) {
	logrus.Printf("validating console service...")
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, recorder, servicesub.DefaultService(operatorConfig))
	if svcErr != nil {
		logrus.Errorf("%q: %v \n", "service", svcErr)
		return nil, false, svcErr
	}
	logrus.Println("service exists and is in the correct state")
	return svc, svcChanged, svcErr
}

// apply route
// - be sure to test that we don't trigger an infinite loop by stomping on the
//   default host name set by the server, or any other values. The ApplyRoute()
//   logic will have to be sound.
// - update to ApplyRoute() once the logic is settled
func SyncRoute(co *consoleOperator, operatorConfig *operatorv1.Console) (*routev1.Route, bool, error) {
	logrus.Printf("validating console route...")
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(operatorConfig))
	// rt, rtChanged, rtErr := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(operatorConfig))
	if rtErr != nil {
		logrus.Errorf("%q: %v \n", "route", rtErr)
		return nil, false, rtErr
	}

	// we will not proceed until the route is valid. this eliminates complexity with the
	// configmap, secret & oauth client as they can be certain they have a host if we pass this point.
	if len(rt.Spec.Host) == 0 {
		// TODO STATUS
		logrus.Errorf("%q: %v \n", "route", rtErr)
		return nil, false, errors.New("waiting on route.spec.host")
	}

	if validatedRoute, changed := routesub.Validate(rt); changed {
		if _, err := co.routeClient.Routes(api.TargetNamespace).Update(validatedRoute); err != nil {
			logrus.Errorf("%q: %v \n", "route", err)
			return nil, false, err
		}
		errMsg := fmt.Errorf("route is invalid, correcting route state")
		logrus.Error(errMsg)
		return nil, true, errMsg
	}
	// only returns the route if we hit the happy path, we cannot make progress w/o the host
	logrus.Println("route exists and is in the correct state")
	return rt, rtIsNew, rtErr
}

func getDeploymentGeneration(co *consoleOperator) int64 {
	deployment, err := co.deploymentClient.Deployments(api.TargetNamespace).Get(deploymentsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}
