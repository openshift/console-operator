// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	versioned "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	internalinterfaces "github.com/open-cluster-management/api/client/cluster/informers/externalversions/internalinterfaces"
	v1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ManagedClusterInformer provides access to a shared informer and lister for
// ManagedClusters.
type ManagedClusterInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.ManagedClusterLister
}

type managedClusterInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewManagedClusterInformer constructs a new informer for ManagedCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewManagedClusterInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredManagedClusterInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredManagedClusterInformer constructs a new informer for ManagedCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredManagedClusterInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ClusterV1().ManagedClusters().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ClusterV1().ManagedClusters().Watch(context.TODO(), options)
			},
		},
		&clusterv1.ManagedCluster{},
		resyncPeriod,
		indexers,
	)
}

func (f *managedClusterInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredManagedClusterInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *managedClusterInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&clusterv1.ManagedCluster{}, f.defaultInformer)
}

func (f *managedClusterInformer) Lister() v1.ManagedClusterLister {
	return v1.NewManagedClusterLister(f.Informer().GetIndexer())
}
