package operator

import (
	"errors"
	"fmt"

	"github.com/openshift/console-operator/pkg/api"

	// 3rd party
	"github.com/sirupsen/logrus"
	// kube
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// openshift

	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
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
func sync_v400(co *consoleOperator, consoleConfig *v1alpha1.Console) (*v1alpha1.Console, bool, error) {
	logrus.Println("running sync loop 4.0.0")

	// track changes, may trigger ripples & update consoleConfig.Status
	toUpdate := false

	rt, rtChanged, rtErr := SyncRoute(co, consoleConfig)
	if rtErr != nil {
		return consoleConfig, toUpdate, rtErr
	}
	toUpdate = toUpdate || rtChanged

	_, svcChanged, svcErr := SyncService(co, consoleConfig)
	if svcErr != nil {
		return consoleConfig, toUpdate, svcErr
	}
	toUpdate = toUpdate || svcChanged

	cm, cmChanged, cmErr := SyncConfigMap(co, consoleConfig, rt)
	if cmErr != nil {
		return consoleConfig, toUpdate, cmErr
	}
	toUpdate = toUpdate || cmChanged

	serviceCAConfigMap, serviceCAConfigMapChanged, serviceCAConfigMapErr := SyncServiceCAConfigMap(co, consoleConfig)
	if serviceCAConfigMapErr != nil {
		return consoleConfig, toUpdate, serviceCAConfigMapErr
	}
	toUpdate = toUpdate || serviceCAConfigMapChanged

	sec, secChanged, secErr := SyncSecret(co, consoleConfig)
	if secErr != nil {
		return consoleConfig, toUpdate, secErr
	}
	toUpdate = toUpdate || secChanged

	_, oauthChanged, oauthErr := SyncOAuthClient(co, consoleConfig, sec, rt)
	if oauthErr != nil {
		return consoleConfig, toUpdate, oauthErr
	}
	toUpdate = toUpdate || oauthChanged

	_, depChanged, depErr := SyncDeployment(co, consoleConfig, cm, serviceCAConfigMap, sec)
	if depErr != nil {
		return consoleConfig, toUpdate, depErr
	}
	toUpdate = toUpdate || depChanged

	// if any of our resources have changed, we should update the consoleConfig.Status. otherwise, skip this step.
	if toUpdate {
		logrus.Infof("sync_v400: to update spec: %v", toUpdate)
		// TODO: set the status.
		// setStatus(consoleConfig.Status, svc, rt, cm, dep, oa, sec)
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
	return consoleConfig, toUpdate, nil
}

func SyncDeployment(co *consoleOperator, consoleConfig *v1alpha1.Console, cm *corev1.ConfigMap, serviceCAConfigMap *corev1.ConfigMap, sec *corev1.Secret) (*appsv1.Deployment, bool, error) {
	logrus.Printf("validating console deployment...")
	defaultDeployment := deploymentsub.DefaultDeployment(consoleConfig, cm, serviceCAConfigMap, sec)
	versionAvailability := &operatorv1alpha1.VersionAvailability{
		Version: consoleConfig.Spec.Version,
	}
	deploymentGeneration := resourcemerge.ExpectedDeploymentGeneration(defaultDeployment, versionAvailability)
	// first establish, do we have a deployment?
	existingDeployment, getDepErr := co.deploymentClient.Deployments(api.TargetNamespace).Get(deploymentsub.Stub().Name, metav1.GetOptions{})

	// if not, create it, first pass
	if apierrors.IsNotFound(getDepErr) {
		logrus.Print("deployment not found, creating new deployment")
		_, depCreated, createdErr := resourceapply.ApplyDeployment(co.deploymentClient, defaultDeployment, deploymentGeneration, true)
		// kill the sync loop
		return nil, depCreated, fmt.Errorf("deployment not found, creating new deployment, create error = %v", createdErr)
	}

	if getDepErr != nil {
		logrus.Errorf("%q: %v \n", "deployment", getDepErr)
		return nil, false, getDepErr
	}

	// otherwise, we may need to update or force a rollout
	if deploymentsub.ResourceVersionsChanged(existingDeployment, cm, serviceCAConfigMap, sec) {
		toUpdate := deploymentsub.UpdateResourceVersions(existingDeployment, cm, serviceCAConfigMap, sec)
		updatedDeployment, depChanged, updateErr := resourceapply.ApplyDeployment(co.deploymentClient, toUpdate, deploymentGeneration, true)
		if updateErr != nil {
			logrus.Errorf("%q: %v \n", "deployment", updateErr)
			return nil, false, updateErr
		}
		return updatedDeployment, depChanged, nil
	}
	logrus.Println("deployment exists and is in the correct state")
	return existingDeployment, false, nil
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func SyncOAuthClient(co *consoleOperator, consoleConfig *v1alpha1.Console, sec *corev1.Secret, rt *routev1.Route) (*oauthv1.OAuthClient, bool, error) {
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
func SyncSecret(co *consoleOperator, consoleConfig *v1alpha1.Console) (*corev1.Secret, bool, error) {
	logrus.Printf("validating oauth secret...")
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
	// if we have to create it, or if the actual Secret is empty/invalid, then we want to return an error
	// to kill this round of the sync loop. The next round can pick up and make progress.
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		_, secChanged, secErr := resourceapply.ApplySecret(co.secretsClient, secretsub.DefaultSecret(consoleConfig, crypto.Random256BitsString()))
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
func SyncConfigMap(co *consoleOperator, consoleConfig *v1alpha1.Console, rt *routev1.Route) (*corev1.ConfigMap, bool, error) {
	logrus.Printf("validating console configmap...")
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, configmapsub.DefaultConfigMap(consoleConfig, rt))
	if cmErr != nil {
		logrus.Errorf("%q: %v \n", "configmap", cmErr)
		return nil, false, cmErr
	}
	logrus.Println("configmap exists and is in the correct state")
	return cm, cmChanged, cmErr
}

// apply service-ca configmap
func SyncServiceCAConfigMap(co *consoleOperator, consoleConfig *v1alpha1.Console) (*corev1.ConfigMap, bool, error) {
	logrus.Printf("validating service-ca configmap...")
	required := configmapsub.DefaultServiceCAConfigMap(consoleConfig)
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
func SyncService(co *consoleOperator, consoleConfig *v1alpha1.Console) (*corev1.Service, bool, error) {
	logrus.Printf("validating console service...")
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, servicesub.DefaultService(consoleConfig))
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
func SyncRoute(co *consoleOperator, consoleConfig *v1alpha1.Console) (*routev1.Route, bool, error) {
	logrus.Printf("validating console route...")
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(consoleConfig))
	// rt, rtChanged, rtErr := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(consoleConfig))
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

//func secretsMatch(secretGetter coreclientv1.SecretsGetter, clientsGetter oauthclientv1.OAuthClientsGetter) bool {
//	secret := getSecret(secretGetter)
//	if secret == nil {
//		return false
//	}
//	oauthClient := getOauthClient(clientsGetter)
//	if oauthClient == nil {
//		return false
//	}
//	return secretAndOauthMatch(secret, oauthClient)
//}

//func getSecret(secretGetter coreclientv1.SecretsGetter) (*corev1.Secret, error) {
//	secret, err := secretGetter.Secrets(controller.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
//	if apierrors.IsNotFound(err) {
//		return nil, nil
//	}
//	return secret, err
//}
//
//func getOauthClient(clientsGetter oauthclientv1.OAuthClientsGetter) (*oauthv1.OAuthClient, error) {
//	oauthClient, err := clientsGetter.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
//	if apierrors.IsNotFound(err) {
//		return nil, nil
//	}
//	return oauthClient, err
// }

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
