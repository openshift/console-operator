package console

import (
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

// this function should handle making the updates needed to take a running
// console & adjust it to fit whatever parameters are passed
func syncConsole(cr *v1alpha1.Console) {
	fmt.Println("TODO: sync console when this is called")
}
