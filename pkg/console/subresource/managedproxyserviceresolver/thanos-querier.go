package managedproxyserviceresolver

import (
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	proxyv1alpha1 "open-cluster-management.io/cluster-proxy/pkg/apis/proxy/v1alpha1"
)

func DefaultThanosQuerierManagedProxyServiceResolver() *proxyv1alpha1.ManagedProxyServiceResolver {
	return &proxyv1alpha1.ManagedProxyServiceResolver{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ManagedProxyServiceResolver",
			APIVersion: proxyv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   api.ThanosQuerierServiceName,
			Labels: util.LabelsForManagedClusterResources(""),
		},
		Spec: proxyv1alpha1.ManagedProxyServiceResolverSpec{
			ManagedClusterSelector: proxyv1alpha1.ManagedClusterSelector{
				Type: proxyv1alpha1.ManagedClusterSelectorTypeClusterSet,
				ManagedClusterSet: &proxyv1alpha1.ManagedClusterSet{
					Name: "global",
				},
			},
			ServiceSelector: proxyv1alpha1.ServiceSelector{
				Type: proxyv1alpha1.ServiceSelectorTypeServiceRef,
				ServiceRef: &proxyv1alpha1.ServiceRef{
					Namespace: api.MonitoringNamespace,
					Name:      api.ThanosQuerierServiceName,
				},
			},
		},
	}
}
