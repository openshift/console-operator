package operator

import (
	"errors"
	"fmt"
	"os"
	"strings"

	// 3rd party
	"github.com/prometheus/client_golang/prometheus"
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
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/subresource/service"
)

var (
	// metric: console_url{url="https://<url>"} 1
	consoleURLMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "console_url",
		Help: "URL of the console exposed on the cluster",
		// one label
	}, []string{"url"})
)

func init() {
	prometheus.MustRegister(consoleURLMetric)
}

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func sync_v400(co *consoleOperator, operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure) error {
	logrus.Println("running sync loop 4.0.0")
	recorder := co.recorder

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	rt, rtChanged, rtErr := SyncRoute(co, operatorConfig)
	if rtErr != nil {
		msg := fmt.Sprintf("%v: %s\n", "route", rtErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return rtErr
	}
	toUpdate = toUpdate || rtChanged

	svc, svcChanged, svcErr := SyncService(co, recorder, operatorConfig)
	if svcErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "service", svcErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return svcErr
	}
	toUpdate = toUpdate || svcChanged

	cm, cmChanged, cmErr := SyncConfigMap(co, recorder, operatorConfig, consoleConfig, infrastructureConfig, rt)
	if cmErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "configmap", cmErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return cmErr
	}
	toUpdate = toUpdate || cmChanged

	serviceCAConfigMap, serviceCAConfigMapChanged, serviceCAConfigMapErr := SyncServiceCAConfigMap(co, operatorConfig)
	if serviceCAConfigMapErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "serviceCAconfigmap", serviceCAConfigMapErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return serviceCAConfigMapErr
	}
	toUpdate = toUpdate || serviceCAConfigMapChanged

	sec, secChanged, secErr := SyncSecret(co, recorder, operatorConfig)
	if secErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "secret", secErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return secErr
	}
	toUpdate = toUpdate || secChanged

	oauthClient, oauthChanged, oauthErr := SyncOAuthClient(co, operatorConfig, sec, rt)
	if oauthErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "oauth", oauthErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return oauthErr
	}
	toUpdate = toUpdate || oauthChanged

	actualDeployment, depChanged, depErr := SyncDeployment(co, recorder, operatorConfig, cm, serviceCAConfigMap, sec, rt)
	if depErr != nil {
		msg := fmt.Sprintf("%q: %v\n", "deployment", depErr)
		logrus.Printf("incomplete sync: %v \n", msg)
		co.ConditionResourceSyncProgressing(operatorConfig, msg)
		return depErr
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
		msg := fmt.Sprintf("%v pods available for console deployment", actualDeployment.Status.ReadyReplicas)
		logrus.Println(msg)
		co.ConditionDeploymentNotAvailable(operatorConfig, msg)
	} else if !routesub.IsAdmitted(rt) {
		logrus.Println("console route is not admitted")
		co.SetStatusCondition(
			operatorConfig,
			operatorv1.OperatorStatusTypeAvailable,
			operatorv1.ConditionFalse,
			"RouteNotAdmitted",
			"console route is not admitted",
		)
	} else {
		co.ConditionDeploymentAvailable(operatorConfig)
	}

	// if we survive the gauntlet, we need to update the console config with the
	// public hostname so that the world can know the console is ready to roll
	logrus.Println("sync_v400: updating console status")
	consoleURL := getConsoleURL(rt)

	if consoleURL == "" {
		err := customerrors.NewSyncError("waiting on route host")
		logrus.Errorf("%q: %v \n", "route", err)
		return err
	}

	if _, err := SyncConsoleConfig(co, consoleConfig, consoleURL); err != nil {
		logrus.Errorf("could not update console config status: %v \n", err)
		return err
	}

	if _, _, err := SyncConsolePublicConfig(co, recorder, consoleURL); err != nil {
		logrus.Errorf("could not update public console config: %v \n", err)
		return err
	}

	defer func() {
		logrus.Printf("sync loop 4.0.0 complete")
		if svcChanged {
			logrus.Printf("\t service changed: %v", svc.GetResourceVersion())
		}
		if rtChanged {
			logrus.Printf("\t route changed: %v", rt.GetResourceVersion())
		}
		if cmChanged {
			logrus.Printf("\t configmap changed: %v", cm.GetResourceVersion())
		}
		if serviceCAConfigMapChanged {
			logrus.Printf("\t service-ca configmap changed: %v", serviceCAConfigMap.GetResourceVersion())
		}
		if secChanged {
			logrus.Printf("\t secret changed: %v", sec.GetResourceVersion())
		}
		if oauthChanged {
			logrus.Printf("\t oauth changed: %v", oauthClient.GetResourceVersion())
		}
		if depChanged {
			logrus.Printf("\t deployment changed: %v", actualDeployment.GetResourceVersion())
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
func (co *consoleOperator) SyncConsoleConfig(consoleConfig *configv1.Console, consoleURL string) (*configv1.Console, error) {
	updated := consoleConfig.DeepCopy()

	// track the URL state in prometheus before we update it
	if consoleConfig.Status.ConsoleURL != consoleURL {
		// not using this URL anymore
		consoleURLMetric.WithLabelValues(consoleConfig.Status.ConsoleURL).Set(0)
	}
	if len(consoleURL) != 0 {
		// only update to new if we have a url
		consoleURLMetric.WithLabelValues(consoleURL).Set(1)
	}

	if updated.Status.ConsoleURL != consoleURL {
		logrus.Infof("updating console.config.openshift.io with url: %v", consoleURL)
		updated.Status.ConsoleURL = consoleURL
	}
	return co.consoleConfigClient.UpdateStatus(updated)
}

func SyncConsoleConfig(co *consoleOperator, consoleConfig *configv1.Console, consoleURL string) (*configv1.Console, error) {
	if consoleConfig.Status.ConsoleURL != consoleURL {
		logrus.Printf("updating console.config.openshift.io with url: %v \n", consoleURL)
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
		logrus.Printf("deployment generation changed from %s to %s \n", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
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
		logrus.Errorf("%q: %v \n", "deployment", applyDepErr)
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
		logrus.Errorf("%q: %v \n", "oauth", customErr)
		return nil, false, customErr
	}
	oauthClient, err := co.oauthClient.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("%q: %v \n", "oauth", err)
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, errors.New("oauth client for console does not exist and cannot be created")
	}
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, host, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.CustomApplyOAuth(co.oauthClient, oauthClient)
	if oauthErr != nil {
		logrus.Errorf("%q: %v \n", "oauth", oauthErr)
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
		logrus.Errorf("%q: %v \n", "secret", err)
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
		logrus.Errorf("managed config error: %v \n", mcErr)
		return nil, false, mcErr
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, managedConfig, infrastructureConfig, rt)
	if err != nil {
		return nil, false, err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		logrus.Errorf("%q: %v \n", "configmap", cmErr)
		return nil, false, cmErr
	}
	if cmChanged {
		logrus.Println("new console config yaml:")
		logrus.Printf("%s \n", cm.Data)
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
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, recorder, servicesub.DefaultService(operatorConfig))
	if svcErr != nil {
		logrus.Errorf("%q: %v \n", "service", svcErr)
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
		logrus.Errorf("%q: %v \n", "route", rtErr)
		return nil, false, rtErr
	}

	// we will not proceed until the route is valid. this eliminates complexity with the
	// configmap, secret & oauth client as they can be certain they have a host if we pass this point.
	host := routesub.GetCanonicalHost(rt)
	if host == "" {
		customErr := customerrors.NewSyncError("waiting on route host")
		logrus.Errorf("%q: %v \n", "route", customErr)
		return nil, false, customErr
	}

	if validatedRoute, changed := routesub.Validate(rt); changed {
		// if validation changed the route, issue an update
		if _, err := co.routeClient.Routes(api.TargetNamespace).Update(validatedRoute); err != nil {
			// error is unexpected, this is a real error
			logrus.Errorf("%q: %v \n", "route", err)
			return nil, false, err
		}
		// abort sync, route changed, let it settle & retry
		customErr := customerrors.NewSyncError("route is invalid, correcting route state")
		logrus.Error(customErr)
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
