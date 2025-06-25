package clidownloads

import (
	// standard lib
	"context"
	"fmt"
	"net/url"
	"time"

	// kube
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	// openshift
	v1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorv1listers "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/library-go/pkg/route/routeapihelpers"

	// informers
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	consoleinformersv1 "github.com/openshift/client-go/console/informers/externalversions/console/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	routev1listers "github.com/openshift/client-go/route/listers/route/v1"

	// clients
	consoleclientv1 "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	controllersutil "github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

type CLIDownloadsSyncController struct {
	// clients
	operatorClient            v1helpers.OperatorClient
	consoleCliDownloadsClient consoleclientv1.ConsoleCLIDownloadInterface
	routeLister               routev1listers.RouteLister
	ingressConfigLister       configlistersv1.IngressLister
	operatorConfigLister      operatorv1listers.ConsoleLister
}

func NewCLIDownloadsSyncController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	// clients
	operatorClient v1helpers.OperatorClient,
	cliDownloadsInterface consoleclientv1.ConsoleCLIDownloadInterface,
	// informers
	operatorConfigInformer operatorinformersv1.ConsoleInformer,
	configInformer configinformer.SharedInformerFactory,
	consoleCLIDownloadsInformers consoleinformersv1.ConsoleCLIDownloadInformer,
	routeInformer routesinformersv1.RouteInformer,
	// events
	recorder events.Recorder,
) factory.Controller {

	ctrl := &CLIDownloadsSyncController{
		// clients
		operatorClient:            operatorClient,
		consoleCliDownloadsClient: cliDownloadsInterface,
		routeLister:               routeInformer.Lister(),
		ingressConfigLister:       configInformer.Config().V1().Ingresses().Lister(),
		operatorConfigLister:      operatorConfigInformer.Lister(),
	}

	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			controllersutil.IncludeNamesFilter(api.ConfigResourceName),
			operatorConfigInformer.Informer(),
			configV1Informers.Ingresses().Informer(),
		).WithFilteredEventsInformers( // console resources
		controllersutil.IncludeNamesFilter(api.OpenShiftConsoleDownloadsRouteName),
		routeInformer.Informer(),
	).WithInformers(
		consoleCLIDownloadsInformers.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("ConsoleCLIDownloadsController", recorder.WithComponentSuffix("console-cli-downloads-controller"))
}

func (c *CLIDownloadsSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing ConsoleCliDownloads custom resources")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping ConsoleCliDownloads custom resources sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting ConsoleCliDownloads custom resources")
		return c.removeCLIDownloads(ctx)
	default:
		return fmt.Errorf("console is in an unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	var (
		downloadsURI *url.URL
		downloadsErr error
	)
	if len(operatorConfig.Spec.Ingress.ClientDownloadsURL) == 0 {
		ingressConfig, err := c.ingressConfigLister.Get(api.ConfigResourceName)
		if err != nil {
			return statusHandler.FlushAndReturn(err)
		}

		activeRouteName := api.OpenShiftConsoleDownloadsRouteName
		routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, ingressConfig, activeRouteName)
		if routeConfig.IsCustomHostnameSet() {
			activeRouteName = api.OpenshiftDownloadsCustomRouteName
		}

		downloadsRoute, downloadsRouteErr := c.routeLister.Routes(api.TargetNamespace).Get(activeRouteName)
		if downloadsRouteErr != nil {
			return downloadsRouteErr
		}

		downloadsURI, _, downloadsErr = routeapihelpers.IngressURI(downloadsRoute, downloadsRoute.Spec.Host)
		if downloadsErr != nil {
			return downloadsErr
		}
	} else {
		downloadsURI, downloadsErr = url.Parse(operatorConfig.Spec.Ingress.ClientDownloadsURL)
		if downloadsErr != nil {
			return fmt.Errorf("failed to parse downloads url: %w", downloadsErr)
		}
	}

	ocConsoleCLIDownloads := PlatformBasedOCConsoleCLIDownloads(downloadsURI.String(), api.OCCLIDownloadsCustomResourceName)
	_, ocCLIDownloadsErrReason, ocCLIDownloadsErr := ApplyCLIDownloads(ctx, c.consoleCliDownloadsClient, ocConsoleCLIDownloads)
	statusHandler.AddCondition(status.HandleDegraded("OCDownloadsSync", ocCLIDownloadsErrReason, ocCLIDownloadsErr))
	if ocCLIDownloadsErr != nil {
		return statusHandler.FlushAndReturn(ocCLIDownloadsErr)
	}

	return statusHandler.FlushAndReturn(nil)
}

func (c *CLIDownloadsSyncController) removeCLIDownloads(ctx context.Context) error {
	defer klog.V(4).Info("finished deleting ConsoleCliDownloads custom resources")
	var errs []error
	errs = append(errs, c.consoleCliDownloadsClient.Delete(ctx, api.OCCLIDownloadsCustomResourceName, metav1.DeleteOptions{}))
	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}

func GetPlatformURL(baseURL, platform, archiveType string) string {
	return fmt.Sprintf("%s/%s/%s", baseURL, platform, archiveType)
}

func PlatformBasedOCConsoleCLIDownloads(host, cliDownloadsName string) *v1.ConsoleCLIDownload {
	baseURL := fmt.Sprintf("%s", util.HTTPS(host))
	platforms := []struct {
		label    string
		key      string
		archType string
	}{
		{"Linux for x86_64", "amd64/linux", "oc.tar"},
		{"Linux for x86_64 - RHEL 8", "amd64/linux", "oc.rhel8.tar"},
		{"Linux for x86_64 - RHEL 9", "amd64/linux", "oc.rhel9.tar"},
		{"Mac for x86_64", "amd64/mac", "oc.zip"},
		{"Windows for x86_64", "amd64/windows", "oc.exe.zip"},
		{"Linux for ARM 64", "arm64/linux", "oc.tar"},
		{"Linux for ARM 64 - RHEL 8", "arm64/linux", "oc.rhel8.tar"},
		{"Linux for ARM 64 - RHEL 9", "arm64/linux", "oc.rhel9.tar"},
		{"Mac for ARM 64", "arm64/mac", "oc.zip"},
		{"Linux for IBM Power, little endian", "ppc64le/linux", "oc.tar"},
		{"Linux for IBM Power, little endian - RHEL 8", "ppc64le/linux", "oc.rhel8.tar"},
		{"Linux for IBM Power, little endian - RHEL 9", "ppc64le/linux", "oc.rhel9.tar"},
		{"Linux for IBM Z", "s390x/linux", "oc.tar"},
		{"Linux for IBM Z - RHEL 8", "s390x/linux", "oc.rhel8.tar"},
		{"Linux for IBM Z - RHEL 9", "s390x/linux", "oc.rhel9.tar"},
	}

	links := []v1.CLIDownloadLink{}
	for _, platform := range platforms {
		links = append(links, v1.CLIDownloadLink{
			Href: GetPlatformURL(baseURL, platform.key, platform.archType),
			Text: fmt.Sprintf("Download oc for %s", platform.label),
		})
	}

	links = append(links, v1.CLIDownloadLink{
		Href: fmt.Sprintf("%s/oc-license", baseURL),
		Text: "LICENSE",
	})

	return &v1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{
			Name: cliDownloadsName,
		},
		Spec: v1.ConsoleCLIDownloadSpec{
			Description: `With the OpenShift command line interface, you can create applications and manage OpenShift projects from a terminal.

The oc binary offers the same capabilities as the kubectl binary, but it is further extended to natively support OpenShift Container Platform features. You can download oc using the following links.
`,
			DisplayName: "oc - OpenShift Command Line Interface (CLI)",
			Links:       links,
		},
	}
}

// TODO: All the custom `Apply*` functions should be at some point be placed into:
// openshift/library-go/pkg/console/resource/resourceapply/core.go
func ApplyCLIDownloads(ctx context.Context, consoleClient consoleclientv1.ConsoleCLIDownloadInterface, requiredCLIDownloads *v1.ConsoleCLIDownload) (*v1.ConsoleCLIDownload, string, error) {
	cliDownloadsName := requiredCLIDownloads.ObjectMeta.Name
	existingCLIDownloads, err := consoleClient.Get(ctx, cliDownloadsName, metav1.GetOptions{})
	existingCLIDownloadsCopy := existingCLIDownloads.DeepCopy()
	if apierrors.IsNotFound(err) {
		actualCLIDownloads, err := consoleClient.Create(ctx, requiredCLIDownloads, metav1.CreateOptions{})
		if err != nil {
			klog.V(4).Infof("error creating %s consoleclidownloads custom resource: %s", cliDownloadsName, err)
			return nil, "FailedCreate", err
		}
		klog.V(4).Infof("%s consoleclidownloads custom resource created", cliDownloadsName)
		return actualCLIDownloads, "", nil
	}
	if err != nil {
		klog.V(4).Infof("error getting %s custom resource: %v", cliDownloadsName, err)
		return nil, "", err
	}
	specSame := equality.Semantic.DeepEqual(existingCLIDownloadsCopy.Spec, requiredCLIDownloads.Spec)
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existingCLIDownloadsCopy.ObjectMeta, requiredCLIDownloads.ObjectMeta)
	if specSame && !*modified {
		klog.V(4).Infof("%s consoleclidownloads custom resource exists and is in the correct state", cliDownloadsName)
		return existingCLIDownloadsCopy, "", nil
	}

	existingCLIDownloadsCopy.Spec = requiredCLIDownloads.Spec
	actualCLIDownloads, err := consoleClient.Update(ctx, existingCLIDownloadsCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.V(4).Infof("error updating %s consoleclidownloads custom resource: %v", cliDownloadsName, err)
		return nil, "FailedUpdate", err
	}
	return actualCLIDownloads, "", nil
}
