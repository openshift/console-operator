package operator

import (
	"errors"
	"fmt"

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
	"github.com/openshift/library-go/pkg/operator/v1helpers"

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
func sync_v400(co *consoleOperator, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console) (*operatorv1.Console, *configv1.Console, bool, error) {
	errors := []error{}
	logrus.Println("running sync loop 4.0.0")
	recorder := co.recorder

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	rt, rtChanged, err := SyncRoute(co, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "route", err))
	}
	toUpdate = toUpdate || rtChanged

	_, svcChanged, err := SyncService(co, recorder, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "service", err))
		// return operatorConfig, consoleConfig, toUpdate, svcErr
	}
	toUpdate = toUpdate || svcChanged

	cm, cmChanged, err := SyncConfigMap(co, recorder, operatorConfig, consoleConfig, rt)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "configmap", err))
		// return operatorConfig, consoleConfig, toUpdate, cmErr
	}
	toUpdate = toUpdate || cmChanged

	serviceCAConfigMap, serviceCAConfigMapChanged, err := SyncServiceCAConfigMap(co, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "serviceCAconfigmap", err))
		// return operatorConfig, consoleConfig, toUpdate, serviceCAConfigMapErr
	}
	toUpdate = toUpdate || serviceCAConfigMapChanged

	sec, secChanged, err := SyncSecret(co, recorder, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "secret", err))
		// return operatorConfig, consoleConfig, toUpdate, secErr
	}
	toUpdate = toUpdate || secChanged

	_, oauthChanged, err := SyncOAuthClient(co, operatorConfig, sec, rt)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "oauth", err))
		// return operatorConfig, consoleConfig, toUpdate, oauthErr
	}
	toUpdate = toUpdate || oauthChanged

	actualDeployment, depChanged, err := SyncDeployment(co, recorder, operatorConfig, cm, serviceCAConfigMap, sec)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "deployment", err))
		// return operatorConfig, consoleConfig, toUpdate, depErr
	}

	// Need to figure out if we also want to check the deployment ReadyReplicas count?
	if actualDeployment.Status.ReadyReplicas > 0 {
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
			Type:               operatorv1.OperatorStatusTypeAvailable,
			Status:             operatorv1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	} else {
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
			Type:               operatorv1.OperatorStatusTypeAvailable,
			Status:             operatorv1.ConditionFalse,
			Reason:             "NoPodsAvailable",
			Message:            "no deployment pods available on any node.",
			LastTransitionTime: metav1.Now(),
		})
	}

	if len(errors) > 0 {
		message := ""
		for _, err := range errors {
			message = message + err.Error() + "\n"
		}
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
			Type:    workloadFailingCondition,
			Status:  operatorv1.ConditionTrue,
			Message: message,
			Reason:  "SyncError",
		})
	} else {
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
			Type:   workloadFailingCondition,
			Status: operatorv1.ConditionFalse,
		})
	}

	toUpdate = toUpdate || depChanged

	logrus.Println("sync_v400: updating console status")
	if updatedConfig, err := SyncConsoleConfig(co, consoleConfig, rt); err != nil {
		logrus.Errorf("Could not update console config status: %v \n", err)
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

	// at this point there should be no existing errors, we survived the sync loop
	// pass back config (updated), and bool indicating change happened so we can update
	// the cluster operator status
	return operatorConfig, consoleConfig, toUpdate, nil
}

func SyncConsoleConfig(co *consoleOperator, consoleConfig *configv1.Console, route *routev1.Route) (*configv1.Console, error) {
	logrus.Printf("Updating console.config.openshift.io with hostname: %v \n", route.Spec.Host)
	consoleConfig.Status.PublicHostname = util.HTTPS(route.Spec.Host)
	return co.consoleConfigClient.UpdateStatus(consoleConfig)
}

func SyncDeployment(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, cm *corev1.ConfigMap, serviceCAConfigMap *corev1.ConfigMap, sec *corev1.Secret) (*appsv1.Deployment, bool, error) {
	logrus.Printf("validating console deployment...")
	requiredDeployment := deploymentsub.DefaultDeployment(operatorConfig, cm, serviceCAConfigMap, sec)
	expectedGeneration := getDeploymentGeneration(co)
	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(co.deploymentClient, recorder, requiredDeployment, expectedGeneration, false)
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
	oauthClient, err := co.oauthClient.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("%q: %v \n", "oauth", err)
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, errors.New("oauth client for console does not exist.")
	}
	// this should take care of all of our syncronization
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, rt, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.ApplyOAuth(co.oauthClient, oauthClient)
	if oauthErr != nil {
		logrus.Errorf("%q: %v \n", "oauth", oauthErr)
		return nil, false, oauthErr
	}
	logrus.Println("oauthclient exists and is in the correct state")
	return oauthClient, oauthChanged, nil
}

// handleSecret() func, return secret, err
// give me a good secret or die
// we want the sync loop to die if we have to create.  thats fine, next pass will fix the rest of things.
// adopt this pattern so we dont have to deal with too much complexity.
func SyncSecret(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console) (*corev1.Secret, bool, error) {
	logrus.Printf("validating oauth secret...")
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
	// if we have to create it, or if the actual Secret is empty/invalid, then we want to return an error
	// to kill this round of the sync loop. The next round can pick up and make progress.
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		_, secChanged, secErr := resourceapply.ApplySecret(co.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
		return nil, secChanged, fmt.Errorf("secret not found, creating new secret, create error = %v", secErr)
	}
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
func SyncConfigMap(co *consoleOperator, recorder events.Recorder, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, rt *routev1.Route) (*corev1.ConfigMap, bool, error) {
	logrus.Printf("validating console configmap...")
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, recorder, configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, rt))
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

func secretAndOauthMatch(secret *corev1.Secret, client *oauthv1.OAuthClient) bool {
	secretString := secretsub.GetSecretString(secret)
	clientSecretString := oauthsub.GetSecretString(client)
	return secretString == clientSecretString
}

func getDeploymentGeneration(co *consoleOperator) int64 {
	deployment, err := co.deploymentClient.Deployments(api.TargetNamespace).Get(deploymentsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}

// the top level config for the console
// this needs the console.status.publicHostname set
func setConsoleConfigStatus() {}

// the operator config expects standard operator status
func setConsoleOperatorConfigStatus() {}
