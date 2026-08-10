package configobservercontroller

import (
	"k8s.io/client-go/tools/cache"

	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/console-operator/pkg/console/configobservation"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	libgoapiserver "github.com/openshift/library-go/pkg/operator/configobserver/apiserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type ConfigObserver struct {
	factory.Controller
}

// NewConfigObserver creates a config observer controller that watches
// the APIServer resource and writes TLS configuration to the Console CR's
// observedConfig field.
func NewConfigObserver(
	operatorClient v1helpers.OperatorClient,
	configInformer configinformers.SharedInformerFactory,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
	eventRecorder events.Recorder,
) *ConfigObserver {
	informers := []factory.Informer{
		operatorClient.Informer(),
		configInformer.Config().V1().APIServers().Informer(),
	}

	c := &ConfigObserver{
		Controller: configobserver.NewConfigObserver(
			"console",
			operatorClient,
			eventRecorder,
			configobservation.Listers{
				APIServerLister_: configInformer.Config().V1().APIServers().Lister(),
				ResourceSync:     resourceSyncer,
				PreRunCachesSynced: []cache.InformerSynced{
					operatorClient.Informer().HasSynced,
					configInformer.Config().V1().APIServers().Informer().HasSynced,
				},
			},
			informers,
			// Observer functions
			libgoapiserver.ObserveTLSSecurityProfile,
		),
	}

	return c
}
