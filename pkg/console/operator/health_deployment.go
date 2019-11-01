package operator

import (
	"errors"
	"fmt"
	"os"

	deploymentv1 "k8s.io/api/apps/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/console-operator/pkg/console/status"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

func (co *consoleOperator) CheckDeploymentHealth(opConfig *operatorv1.Console, deployment *deploymentv1.Deployment, toUpdate bool) {
	status.HandleAvailable(func() (conf *operatorv1.Console, prefix string, reason string, err error) {
		prefix = "Deployment"
		if !deploymentsub.IsReady(deployment) {
			return opConfig, prefix, "InsufficientReplicas", errors.New(fmt.Sprintf("%v pods available for console deployment", deployment.Status.ReadyReplicas))
		}
		if !deploymentsub.IsReadyAndUpdated(deployment) {
			return opConfig, prefix, "FailedUpdate", errors.New(fmt.Sprintf("%v replicas ready at version %s", deployment.Status.ReadyReplicas, os.Getenv("RELEASE_VERSION")))
		}
		return opConfig, prefix, "", nil
	}())

	status.HandleProgressing(opConfig, "SyncLoopRefresh", "InProgress", func() error {
		if toUpdate {
			return errors.New("Changes made during sync updates, additional sync expected.")
		}
		version := os.Getenv("RELEASE_VERSION")
		if !deploymentsub.IsAvailableAndUpdated(deployment) {
			return errors.New(fmt.Sprintf("Working toward version %s", version))
		}
		if co.versionGetter.GetVersions()["operator"] != version {
			co.versionGetter.SetVersion("operator", version)
		}
		return nil
	}())
}
