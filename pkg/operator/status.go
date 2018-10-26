package operator

import (
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

// TODO: needs Route, And EVERYTHING to see if it all works or not.
// Update the status on the Console CRD
// Should prob event as well?
// https://github.com/openshift/elasticsearch-operator/blob/master/pkg/k8shandler/status.go
func UpdateStatus(console v1alpha1.Console) (v1alpha1.Console, error) {
	fmt.Println("TODO: update status on the CRD")

	// All API types should have a DeepCopy() fn
	// this is generated code, if types.go changs I must
	// regeneate the k8s stuff.
	// copy := console.DeepCopy()
}
