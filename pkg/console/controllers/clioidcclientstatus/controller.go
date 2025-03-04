package clioidcclientstatus

import (
	"context"
	"fmt"
	"time"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	authnsub "github.com/openshift/console-operator/pkg/console/subresource/authentication"
)

// cliOIDCClientStatusController:
//
//	writes:
//	- authentication.config.openshift.io/cluster .status.oidcClients:
//		- componentName=cli
//		- componentNamespace=openshift-console
//		- currentOIDCClients
//		- conditions:
//			- Available
//			- Progressing
//			- Degraded
//	- consoles.operator.openshift.io/cluster .status.conditions:
//		- type=CLIOIDCClientStatusProgressing
//		- type=CLIOIDCClientStatusDegraded
//		- type=CLIAuthStatusHandlerProgressing
//		- type=CLIAuthStatusHandlerDegraded
type cliOIDCClientStatusController struct {
	authnLister                configv1listers.AuthenticationLister
	authStatusHandler          *status.AuthStatusHandler
	operatorClient             v1helpers.OperatorClient
	statusHandler              status.StatusHandler
	externalOIDCFeatureEnabled bool
}

func NewCLIOIDCClientStatusController(
	operatorClient v1helpers.OperatorClient,
	authnInformer configv1informers.AuthenticationInformer,
	authenticationClient configv1client.AuthenticationInterface,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	externalOIDCFeatureEnabled bool,
	recorder events.Recorder,
) factory.Controller {
	c := &cliOIDCClientStatusController{
		authnLister:                authnInformer.Lister(),
		authStatusHandler:          status.NewAuthStatusHandler(authenticationClient, api.CLIOIDCClientComponentName, api.TargetNamespace, "CLIOIDCClientStatusController"),
		externalOIDCFeatureEnabled: externalOIDCFeatureEnabled,
		operatorClient:             operatorClient,
	}
	return factory.New().
		WithSync(c.sync).
		ResyncEvery(wait.Jitter(time.Minute, 1.0)).
		WithInformers(
			authnInformer.Informer(),
			consoleOperatorInformer.Informer(),
		).
		ToController("CLIOIDCClientStatusController", recorder.WithComponentSuffix("CLIOIDCClientStatusController"))
}

func (c *cliOIDCClientStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	c.statusHandler = status.NewStatusHandler(c.operatorClient)
	return util.HandleManagementState(ctx, c, c.operatorClient)
}

func (c *cliOIDCClientStatusController) HandleUnmanaged(ctx context.Context) error {
	klog.V(4).Info("Console is in unmanaged state, skipping CLI OIDC client status sync.")
	return nil
}

func (c *cliOIDCClientStatusController) HandleRemoved(ctx context.Context) error {
	klog.V(4).Info("Console is in removed state, skipping CLI OIDC client status sync.")
	return nil
}

func (c *cliOIDCClientStatusController) HandleManaged(ctx context.Context) error {
	klog.V(4).Info("Console is in managed state, syncing CLI OIDC client status.")

	if !c.externalOIDCFeatureEnabled {
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("CLIOIDCClientStatus", "", nil))
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("CLIAuthStatusHandler", "", nil))
		return c.statusHandler.FlushAndReturn(nil)
	}

	authnConfig, err := c.authnLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	if authnConfig.Spec.Type != configv1.AuthenticationTypeOIDC {
		// If the authentication type is not "OIDC", set the CurrentOIDCClient
		// on the authStatusHandler to an empty string. This is necessary during a
		// scenario where the authentication type goes from OIDC to non-OIDC because
		// the CurrentOIDCClient would have been set while the authentication type was OIDC.
		// If the CurrentOIDCClient value isn't reset on this transition the authStatusHandler
		// will think OIDC is still configured and attempt to update the OIDC client in the
		// status to have an empty providerName and issuerURL, violating the validations
		// on the Authentication CRD as seen in https://issues.redhat.com/browse/OCPBUGS-44953
		c.authStatusHandler.WithCurrentOIDCClient("")

		applyErr := c.authStatusHandler.Apply(ctx, authnConfig)
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("CLIAuthStatusHandler", "FailedApply", applyErr))

		// reset the other condition set by this controller
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("CLIOIDCClientStatus", "", nil))
		return c.statusHandler.FlushAndReturn(applyErr)
	}

	// we need to keep track of errors during the sync so that we can requeue
	// if any occur
	var errs []error
	syncErr := c.syncOIDCCLient(authnConfig)
	c.statusHandler.AddConditions(
		status.HandleProgressingOrDegraded(
			"CLIOIDCClientStatus", "CLIOIDCClientStatusSyncFailed",
			syncErr,
		),
	)
	if syncErr != nil {
		errs = append(errs, syncErr)
	}

	applyErr := c.authStatusHandler.Apply(ctx, authnConfig)
	c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("CLIAuthStatusHandler", "FailedApply", applyErr))
	if applyErr != nil {
		errs = append(errs, applyErr)
	}

	if len(errs) > 0 {
		return c.statusHandler.FlushAndReturn(factory.SyntheticRequeueError)
	}
	return c.statusHandler.FlushAndReturn(nil)
}

func (c *cliOIDCClientStatusController) syncOIDCCLient(authnConfig *configv1.Authentication) error {
	_, clientConfig := authnsub.GetOIDCClientConfig(authnConfig, api.TargetNamespace, api.CLIOIDCClientComponentName)
	if clientConfig == nil {
		c.authStatusHandler.WithCurrentOIDCClient("")
		c.authStatusHandler.Unavailable("CLIOIDCClientStatus", "no CLI OIDC client spec found")
		return nil
	}
	if len(clientConfig.ClientID) == 0 {
		return fmt.Errorf("no ID set on CLI OIDC client spec")
	}
	c.authStatusHandler.WithCurrentOIDCClient(clientConfig.ClientID)
	c.authStatusHandler.Available("CLIOIDCConfigAvailable", "")
	return nil
}
