package util

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1lister "github.com/openshift/client-go/config/listers/config/v1"
	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	oauthlistersv1 "github.com/openshift/client-go/oauth/listers/oauth/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type InformerWithSwitch struct {
	informer         oauthinformersv1.OAuthClientInformer
	authnLister      configv1lister.AuthenticationLister
	switchController factory.Controller
	parentCtx        context.Context
	runCtx           context.Context
	stopFunc         func()
}

// since the SwitchedInformer can be stopped, waiting for its cache to sync (via HasSynced)
// can lead to timeouts, as a stopped informer will never sync; we override the HasSynced
// method to always return true; clients should explicitly call cache.WaitForCacheSync
type alwaysSyncedInformer struct {
	isRunning func() bool
	cache.SharedIndexInformer
}

func (s *alwaysSyncedInformer) HasSynced() bool {
	if s.isRunning() {
		return s.SharedIndexInformer.HasSynced()
	}
	return true
}

func tweakListOptionsForOAuthInformer(options *metav1.ListOptions) {
	options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
}

func NewSwitchedInformer(
	ctx context.Context,
	oauthClient authclient.Interface,
	resync time.Duration,
	authnInformer configv1informers.AuthenticationInformer,
	recorder events.Recorder,
) *InformerWithSwitch {
	// oauthclients are not namespaced
	oauthInformers := oauthinformers.NewSharedInformerFactoryWithOptions(
		oauthClient,
		resync,
		oauthinformers.WithTweakListOptions(tweakListOptionsForOAuthInformer),
	)

	s := &InformerWithSwitch{
		parentCtx:   ctx,
		informer:    oauthInformers.Oauth().V1().OAuthClients(),
		authnLister: authnInformer.Lister(),
	}

	s.switchController = factory.New().
		WithSync(s.sync).
		WithInformers(authnInformer.Informer()).
		ToController("InformerWithSwitchController", recorder.WithComponentSuffix("informer-with-switch-controller"))

	return s
}

func (s *InformerWithSwitch) Informer() cache.SharedIndexInformer {
	return &alwaysSyncedInformer{
		isRunning:           func() bool { return s.runCtx != nil },
		SharedIndexInformer: s.informer.Informer(),
	}
}

func (s *InformerWithSwitch) Lister() oauthlistersv1.OAuthClientLister {
	return s.informer.Lister()
}

func (s *InformerWithSwitch) Start(stopCh <-chan struct{}) {
	go s.switchController.Run(s.parentCtx, 1)
	go func() {
		<-stopCh
		s.stop()
	}()
}

func (s *InformerWithSwitch) ensureRunning() {
	if s.runCtx != nil {
		return
	}

	s.runCtx, s.stopFunc = context.WithCancel(s.parentCtx)
	go s.informer.Informer().Run(s.runCtx.Done())
}

func (s *InformerWithSwitch) stop() {
	if s.runCtx == nil {
		return
	}

	s.stopFunc()
	s.runCtx = nil
	s.stopFunc = nil
}

func (s *InformerWithSwitch) sync(ctx context.Context, _ factory.SyncContext) error {
	authnConfig, err := s.authnLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	switch authnConfig.Spec.Type {
	// We don't disable auth since the internal OAuth server is not disabled even with auth type 'None'.
	case "", configv1.AuthenticationTypeIntegratedOAuth, configv1.AuthenticationTypeNone:
		klog.V(4).Infof("authentication type '%s'; starting OAuth clients informer", authnConfig.Spec.Type)
		s.ensureRunning()

	case configv1.AuthenticationTypeOIDC:
		klog.V(4).Infof("authentication type '%s'; stopping OAuth clients informer", authnConfig.Spec.Type)
		s.stop()

	default:
		return fmt.Errorf("unexpected authentication type: %s", authnConfig.Spec.Type)
	}

	return nil
}
