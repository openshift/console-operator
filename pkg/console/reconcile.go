package console

import (
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func Reconcile (cr *v1alpha1.Console) (err error) {
	// SetDefaults() should be non-destructive, only
	// setting defaults if they do not exist.  This
	// should be safe to call every time Reconcile()
	// is called.
	// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/reconcile.go#L18
	changed := cr.SetDefaults()
	fmt.Println("Defaults updated:", changed)
	// this function will eventually do more things.
	// for now, it just needs to deploy the console.
	// eventually,
	// will want to deploy each of the components
	//
	// then, will want to check status & do actual
	// reconciliation work.
	err = deployConsole(cr) // syncConsole(cr) for sync?
	if err != nil {
		return err
	}
	return nil
}