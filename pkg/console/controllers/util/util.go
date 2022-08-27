package util

import (
	//k8s
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	//github
	"github.com/blang/semver"
	"github.com/openshift/library-go/pkg/controller/factory"
)

// Return func which returns true if obj name is in names
func IncludeNamesFilter(names ...string) factory.EventFilterFunc {
	nameSet := sets.NewString(names...)
	return func(obj interface{}) bool {
		metaObj := obj.(metav1.Object)
		return nameSet.Has(metaObj.GetName())
	}
}

// Inverse of IncludeNamesFilter
func ExcludeNamesFilter(names ...string) factory.EventFilterFunc {
	return func(obj interface{}) bool {
		return !IncludeNamesFilter(names...)(obj)
	}
}

// Return a func which returns true if obj matches on every label in labels
// (i.e for each key in labels map, obj.metadata.labels[key] is equal to labels[key])
func LabelFilter(labels map[string]string) factory.EventFilterFunc {
	return func(obj interface{}) bool {
		metaObj := obj.(metav1.Object)
		objLabels := metaObj.GetLabels()
		for k, v := range labels {
			if objLabels[k] != v {
				return false
			}
		}
		return true
	}
}

// contains checks if a string is present in a slice
func SliceContains(s []string, value string) bool {
	for _, v := range s {
		if v == value {
			return true
		}
	}
	return false
}

func IsSupportedVersion(productVersion string) bool {
	version, err := semver.Parse(productVersion)
	if err != nil {
		klog.V(4).Infof("unable to parse %q version", productVersion)
		return false
	}
	return version.Compare(semver.MustParse("4.0.0")) == 1
}
