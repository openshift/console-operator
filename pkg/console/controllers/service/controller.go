package service

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	// key is basically irrelevant
	controllerWorkQueueKey = "service-sync-work-queue-key"
	controllerName         = "ConsoleServiceSyncController"
)

// ctrl just needs the clients so it can make requests
// the informers will automatically notify it of changes
// and kick the sync loop
type ServiceSyncController struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	// live clients, we dont need listers w/caches
	serviceClient coreclientv1.ServicesGetter
	// names
	targetNamespace string
	serviceName     string
	// events
	cachesToSync []cache.InformerSynced
	queue        workqueue.RateLimitingInterface
	recorder     events.Recorder
	// context
	ctx context.Context
}

// factory func needs clients and informers
// informers to start them up, clients to pass
func NewServiceSyncController(
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	corev1Client coreclientv1.CoreV1Interface,
	// informers
	operatorConfigInformer operatorinformersv1.ConsoleInformer,
	serviceInformer coreinformersv1.ServiceInformer,
	// names
	targetNamespace string,
	serviceName string,
	// events
	recorder events.Recorder,
	// context
	ctx context.Context,
) *ServiceSyncController {

	corev1Client.Services(targetNamespace)

	ctrl := &ServiceSyncController{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		serviceClient:        corev1Client,
		// names
		targetNamespace: targetNamespace,
		serviceName:     serviceName,
		// events
		recorder:     recorder,
		cachesToSync: nil,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		ctx:          ctx,
	}

	operatorClient.Informer().AddEventHandler(ctrl.newEventHandler())
	operatorConfigInformer.Informer().AddEventHandler(ctrl.newEventHandler())
	serviceInformer.Informer().AddEventHandler(ctrl.newEventHandler())

	ctrl.cachesToSync = append(ctrl.cachesToSync,
		operatorClient.Informer().HasSynced,
		operatorConfigInformer.Informer().HasSynced,
		serviceInformer.Informer().HasSynced,
	)

	return ctrl
}

func (c *ServiceSyncController) sync() error {
	startTime := time.Now()
	klog.V(4).Infof("started syncing service %q (%v)", c.serviceName, startTime)
	defer klog.V(4).Infof("finished syncing service %q (%v)", c.serviceName, time.Since(startTime))
	operatorConfig, err := c.operatorConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing service")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping service sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting service")
		if err = c.removeService(api.OpenshiftConsoleRedirectServiceName); err != nil {
			return err
		}
		return c.removeService(c.serviceName)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	requiredSvc := service.DefaultService(updatedOperatorConfig)
	_, _, svcErr := resourceapply.ApplyService(c.serviceClient, c.recorder, requiredSvc)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ServiceSync", "FailedApply", svcErr))
	if svcErr != nil {
		return statusHandler.FlushAndReturn(svcErr)
	}

	redirectSvcErrReason, redirectSvcErr := c.SyncRedirectService(updatedOperatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("RedirectServiceSync", redirectSvcErrReason, redirectSvcErr))

	return statusHandler.FlushAndReturn(redirectSvcErr)
}

func (c *ServiceSyncController) SyncRedirectService(operatorConfcig *operatorsv1.Console) (string, error) {
	if !routesub.IsCustomRouteSet(operatorConfcig) {
		if err := c.removeService(api.OpenshiftConsoleRedirectServiceName); err != nil {
			return "FailedDelete", err
		}
		return "", nil
	}
	requiredRedirectService := service.RedirectService(operatorConfcig)
	_, _, redirectSvcErr := resourceapply.ApplyService(c.serviceClient, c.recorder, requiredRedirectService)
	if redirectSvcErr != nil {
		return "FailedApply", redirectSvcErr
	}
	return "", redirectSvcErr
}

func (c *ServiceSyncController) removeService(serviceName string) error {
	err := c.serviceClient.Services(c.targetNamespace).Delete(c.ctx, serviceName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *ServiceSyncController) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof("starting %v", controllerName)
	defer klog.Infof("shutting down %v", controllerName)
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		klog.Infoln("caches did not sync")
		runtime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}
	// only start one worker
	go wait.Until(c.runWorker, time.Second, stopCh)
	<-stopCh
}

func (c *ServiceSyncController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ServiceSyncController) processNextWorkItem() bool {
	processKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(processKey)
	err := c.sync()
	if err == nil {
		c.queue.Forget(processKey)
		return true
	}
	runtime.HandleError(fmt.Errorf("%v failed with : %v", processKey, err))
	c.queue.AddRateLimited(processKey)
	return true
}

func (c *ServiceSyncController) newEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}
