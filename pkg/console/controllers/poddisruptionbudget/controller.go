package pdb

import (
	"context"
	"fmt"
	"time"

	// k8s
	"k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1 "k8s.io/client-go/informers/policy/v1"
	policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/klog/v2"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"

	"github.com/openshift/library-go/pkg/operator/events"
)

type PodDisruptionBudgetController struct {
	pdbName              string
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	pdbClient            policyv1client.PodDisruptionBudgetsGetter
}

func NewPodDisruptionBudgetController(
	// name of the PDB instance
	pdbName string,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	pdbClient policyv1client.PodDisruptionBudgetsGetter,
	// informer
	pdbInformer policyv1.PodDisruptionBudgetInformer,
	//events
	recorder events.Recorder,
) factory.Controller {

	ctrl := &PodDisruptionBudgetController{
		pdbName:              pdbName,
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		pdbClient:            pdbClient,
	}

	return factory.New().
		WithFilteredEventsInformers(
			util.IncludeNamesFilter(pdbName),
			pdbInformer.Informer(),
		).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("PodDisruptionBudgetController", recorder.WithComponentSuffix(fmt.Sprintf("%s-pdb-controller", pdbName)))
}

func (c *PodDisruptionBudgetController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infof("console-operator is in a managed state: syncing %q pdb", c.pdbName)
	case operatorsv1.Unmanaged:
		klog.V(4).Infof("console-operator is in an unmanaged state: skipping pdb %q sync", c.pdbName)
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: deleting %q pdb", c.pdbName)
		if err = c.removePodDisruptionBudget(ctx); err != nil {
			return err
		}
		return c.removePodDisruptionBudget(ctx)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	requiredPDB := c.getDefaultPodDisruptionBudget()
	_, _, pdbErr := resourceapply.ApplyPodDisruptionBudget(ctx, c.pdbClient, controllerContext.Recorder(), requiredPDB)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("PDBSync", "FailedApply", pdbErr))
	if pdbErr != nil {
		return statusHandler.FlushAndReturn(pdbErr)
	}
	return statusHandler.FlushAndReturn(pdbErr)
}

// Remove the PDB instance the controller is managing
func (c *PodDisruptionBudgetController) removePodDisruptionBudget(ctx context.Context) error {
	err := c.pdbClient.PodDisruptionBudgets(api.OpenShiftConsoleNamespace).Delete(ctx, c.pdbName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// Load manifests and create the PDBs
func (c *PodDisruptionBudgetController) getDefaultPodDisruptionBudget() *v1.PodDisruptionBudget {
	pdb := resourceread.ReadPodDisruptionBudgetV1OrDie(assets.MustAsset(fmt.Sprintf("pdb/%s-pdb.yaml", c.pdbName)))
	return pdb
}
