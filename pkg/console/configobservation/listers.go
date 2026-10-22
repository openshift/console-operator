package configobservation

import (
	"k8s.io/client-go/tools/cache"

	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
)

type Listers struct {
	APIServerLister_   configlistersv1.APIServerLister
	ResourceSync       resourcesynccontroller.ResourceSyncer
	PreRunCachesSynced []cache.InformerSynced

	// TLSGroupsEnabled gates observation of servingInfo.tlsGroups. The APIServer
	// groups field is itself gated by the TLSGroupPreferences feature gate, so
	// the console observer must not manage tlsGroups when the gate is disabled.
	TLSGroupsEnabled bool
	// FIPSEnabled is resolved once at startup. FIPS mode is an immutable property
	// of the node's kernel, so it is not re-read per observation.
	FIPSEnabled bool
}

func (l Listers) APIServerLister() configlistersv1.APIServerLister {
	return l.APIServerLister_
}

func (l Listers) TLSGroupPreferencesEnabled() bool {
	return l.TLSGroupsEnabled
}

func (l Listers) FIPSModeEnabled() bool {
	return l.FIPSEnabled
}

func (l Listers) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return l.ResourceSync
}

func (l Listers) PreRunHasSynced() []cache.InformerSynced {
	return l.PreRunCachesSynced
}
