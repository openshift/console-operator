package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/library-go/pkg/controller/factory"
)

func NamesFilter(names ...string) factory.EventFilterFunc {
	nameSet := sets.NewString(names...)
	return func(obj interface{}) bool {
		metaObj := obj.(metav1.Object)
		if nameSet.Has(metaObj.GetName()) {
			return true
		}
		return false
	}
}

func ExcludeName(name string) factory.EventFilterFunc {
	return func(obj interface{}) bool {
		metaObj := obj.(metav1.Object)
		return metaObj.GetName() != name
	}
}
