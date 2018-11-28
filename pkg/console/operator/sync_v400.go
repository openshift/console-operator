package operator

import (
	// 3rd party
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	// kube
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errutil "k8s.io/apimachinery/pkg/util/errors"
	// openshift
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	// operator
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	configmapsub "github.com/openshift/console-operator/pkg/console/resource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/resource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/resource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/resource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/resource/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/resource/service"
)

func sync_v400(co *ConsoleOperator, consoleConfig *v1alpha1.Console) (*v1alpha1.Console, error) {
	// aggregate
	allErrors := []error{}
	// track changes, may triggler ripples & update consoleConfig.Status
	toUpdate := false

	// apply service
	_, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, servicesub.DefaultService(consoleConfig))
	if svcErr != nil {
		logrus.Errorf("%q: %v \n", "service", svcErr)
		allErrors = append(allErrors, svcErr)
	}
	toUpdate = toUpdate || svcChanged

	// apply route
	// - be sure to test that we don't trigger an infinite loop by stomping on the
	//   default host name set by the server, or any other values. The ApplyRoute()
	//   logic will have to be sound.
	// - update to ApplyRoute() once the logic is settled
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(consoleConfig))
	// rt, rtChanged, rtErr := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(consoleConfig))
	if rtErr != nil {
		logrus.Errorf("%q: %v \n", "route", rtErr)
		allErrors = append(allErrors, rtErr)
	}
	toUpdate = toUpdate || rtIsNew

	// apply configmap (needs route)
	_, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, configmapsub.DefaultConfigMap(consoleConfig, rt))
	if cmErr != nil {
		logrus.Errorf("%q: %v \n", "configmap", cmErr)
		allErrors = append(allErrors, cmErr)
	}
	toUpdate = toUpdate || cmChanged

	// TODO: clean up, not fond of the scoping issues here
	// - the deployment needs to know about the change value in order to address updates,
	//   but the wrapper is needed to avoid triggering loops unnecessarily
	secretChanged := false
	// oauthChanged := false
	if !secretsMatch(co.secretsClient, co.oauthClient) {
		// shared secret bits
		// sharedOAuthSecretBits := crypto.RandomBits(256)
		sharedOAuthSecretBits := crypto.Random256BitsString()

		// apply oauth (needs route)
		defaultOauthClient := oauthsub.RegisterConsoleToOAuthClient(oauthsub.DefaultOauthClient(), rt, sharedOAuthSecretBits)
		_, oauthChanged, oauthErr := oauthsub.ApplyOAuth(co.oauthClient, defaultOauthClient)
		if oauthErr != nil {
			logrus.Errorf("%q: %v \n", "oauthclient", oauthErr)
			allErrors = append(allErrors, oauthErr)
		}
		toUpdate = toUpdate || oauthChanged

		// apply secret
		_, secretChanged, secErr := resourceapply.ApplySecret(co.secretsClient, secretsub.DefaultSecret(consoleConfig, sharedOAuthSecretBits))
		if secErr != nil {
			logrus.Errorf("sec error: %v", secErr)
			logrus.Errorf("%q: %v \n", "secret", secErr)
			allErrors = append(allErrors, secErr)
		}
		toUpdate = toUpdate || secretChanged
	}

	// we don't want to thrash our deployment, but we also need to force rollout the pod whenever anything critical changes
	defaultDeployment := deploymentsub.DefaultDeployment(consoleConfig)
	versionAvailability := &operatorv1alpha1.VersionAvailability{
		Version: consoleConfig.Spec.Version,
	}
	deploymentGeneration := resourcemerge.ExpectedDeploymentGeneration(defaultDeployment, versionAvailability)
	// if configMap or secrets change, we need to deploy a new pod
	redeployPods := cmChanged || secretChanged
	_, depChanged, depErr := resourceapply.ApplyDeployment(co.deploymentClient, defaultDeployment, deploymentGeneration, redeployPods)
	if depErr != nil {
		logrus.Errorf("%q: %v \n", "deployment", depErr)
		allErrors = append(allErrors, depErr)
	}
	toUpdate = toUpdate || depChanged

	// handy debugging block
	//logrus.Printf("service changed: %v \n", svcChanged)
	//logrus.Printf("route is new: %v \n", rtIsNew)
	//logrus.Printf("configMap changed: %v \n", cmChanged)
	//logrus.Printf("secret changed: %v \n", secretChanged)
	//logrus.Printf("oauth changed: %v \n", oauthChanged)
	//logrus.Printf("deployment changed: %v \n", depChanged)
	//logrus.Println("------------")

	// if any of our resources have changed, we should update the consoleConfig.Status. otherwise, skip this step.
	if toUpdate {
		logrus.Infof("Sync_v400: To update Spec? %v", toUpdate)
		// TODO: set the status.
		// setStatus(consoleConfig.Status, svc, rt, cm, dep, oa, sec)
	}

	return consoleConfig, errutil.FilterOut(errutil.NewAggregate(allErrors), apierrors.IsNotFound)
}

func secretsMatch(secretGetter coreclientv1.SecretsGetter, clientsGetter oauthclientv1.OAuthClientsGetter) bool {
	secret, err := secretGetter.Secrets(controller.TargetNamespace).Get(deploymentsub.ConsoleOauthConfigName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	oauthClient, err := clientsGetter.OAuthClients().Get(controller.OpenShiftConsoleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}

	return secretAndOauthMatch(secret, oauthClient)
}

func secretAndOauthMatch(secret *corev1.Secret, client *oauthv1.OAuthClient) bool {
	secretString := secretsub.GetSecretString(secret)
	clientSecretString := oauthsub.GetSecretString(client)
	return secretString == clientSecretString
}

// update status on CR
// pass in each of the above resources, possibly the
// boolean for "changed" as well, and then assign a status
// on the CR.Status.  Be sure to account for a nil return value
// as some of our resources (oauthlient, configmap) may simply
// not be possible to create if they lack the appropriate inputs.
// in this case, the Status should CLEARLY indicate this to the user.
// Once the resource is correctly created, the status should be updated
// again.  This should be pretty simple and straightforward work.
// update cluster operator status... i believe this
// should be automatic so long as the CR.Status is
// properly filled out with the appropriate values.
func setStatus(cs v1alpha1.ConsoleStatus, svc *corev1.Service, rt *routev1.Route, cm *corev1.ConfigMap, dep *appsv1.Deployment, oa *oauthv1.OAuthClient, sec *corev1.Secret) {
	// TODO: handle custom hosts as well
	if rt.Spec.Host != "" {
		cs.DefaultHostName = rt.Spec.Host
		logrus.Printf("stats.DefaultHostName set to %v", rt.Spec.Host)
	} else {
		cs.DefaultHostName = ""
		logrus.Printf("stats.DefaultHostName set to %v", "")
	}

	if secretAndOauthMatch(sec, oa) {
		cs.OAuthSecret = "valid"
		logrus.Printf("status.OAuthSecret is valid")
	} else {
		cs.OAuthSecret = "mismatch"
		logrus.Printf("status.OAuthSecret is mismatch")
	}

}
