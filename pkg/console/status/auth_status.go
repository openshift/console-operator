package status

import (
	"context"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	configv1ac "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

const (
	conditionTypeDegraded    = "Degraded"
	conditionTypeProgressing = "Progressing"
	conditionTypeAvailable   = "Available"
)

type AuthStatusHandler struct {
	client             configv1client.AuthenticationInterface
	componentName      string
	componentNamespace string
	fieldManager       string
	conditionsToApply  map[string]*metav1.Condition
	currentClientID    string
}

// NewAuthStatusHandler creates a handler for updating the Authentication.config.openshift.io
// status with information about the expected and currently used OIDC client.
// Not thread safe, only use in controllers with a single worker!
func NewAuthStatusHandler(authnClient configv1client.AuthenticationInterface, componentName, componentNamespace, fieldManager string) *AuthStatusHandler {
	return &AuthStatusHandler{
		client:             authnClient,
		componentName:      componentName,
		componentNamespace: componentNamespace,
		fieldManager:       fieldManager,
		conditionsToApply:  map[string]*metav1.Condition{},
	}
}

// Degraded sets the Degraded condition to True and Progressing to False
func (c *AuthStatusHandler) Degraded(reason, message string) {
	now := metav1.Now()
	c.setCondition(conditionTypeProgressing, metav1.ConditionFalse, reason, message, now)
	c.setCondition(conditionTypeDegraded, metav1.ConditionTrue, reason, message, now)
}

// Progressing sets the Progressing condition to True and Degraded to False
func (c *AuthStatusHandler) Progressing(reason, message string) {
	now := metav1.Now()
	c.setCondition(conditionTypeProgressing, metav1.ConditionTrue, reason, message, now)
	c.setCondition(conditionTypeDegraded, metav1.ConditionFalse, reason, message, now)
}

// Unavailable sets all conditions to False
func (c *AuthStatusHandler) Unavailable(reason, message string) {
	now := metav1.Now()
	c.setCondition(conditionTypeAvailable, metav1.ConditionFalse, reason, message, now)
	c.setCondition(conditionTypeProgressing, metav1.ConditionFalse, reason, message, now)
	c.setCondition(conditionTypeDegraded, metav1.ConditionFalse, reason, message, now)
}

// Available sets the Available condition to True, and Progressing and Degraded to False
func (c *AuthStatusHandler) Available(reason, message string) {
	now := metav1.Now()
	c.setCondition(conditionTypeAvailable, metav1.ConditionTrue, reason, message, now)
	c.setCondition(conditionTypeProgressing, metav1.ConditionFalse, reason, message, now)
	c.setCondition(conditionTypeDegraded, metav1.ConditionFalse, reason, message, now)
}

func (c *AuthStatusHandler) setCondition(conditionType string, status metav1.ConditionStatus, reason, message string, ts metav1.Time) {
	if c.conditionsToApply[conditionType] == nil {
		c.conditionsToApply[conditionType] = &metav1.Condition{Type: string(conditionType)}
	}

	c.conditionsToApply[conditionType].Status = status
	c.conditionsToApply[conditionType].Reason = reason
	c.conditionsToApply[conditionType].Message = message
	c.conditionsToApply[conditionType].LastTransitionTime = ts
}

func (c *AuthStatusHandler) WithCurrentOIDCClient(currentClientID string) {
	c.currentClientID = currentClientID
}

func (c *AuthStatusHandler) Apply(ctx context.Context, authnConfig *configv1.Authentication) error {
	defer func() {
		c.conditionsToApply = map[string]*metav1.Condition{}
	}()

	applyConfig, err := configv1ac.ExtractAuthenticationStatus(authnConfig, c.fieldManager)
	if err != nil {
		return err
	}

	clientStatus := &configv1ac.OIDCClientStatusApplyConfiguration{
		ComponentName:      &c.componentName,
		ComponentNamespace: &c.componentNamespace,
	}

	if len(c.currentClientID) > 0 {
		var providerName, providerIssuerURL string
		if len(authnConfig.Spec.OIDCProviders) > 0 {
			providerName = authnConfig.Spec.OIDCProviders[0].Name
			providerIssuerURL = authnConfig.Spec.OIDCProviders[0].Issuer.URL
		}

		clientStatus.WithCurrentOIDCClients(
			&configv1ac.OIDCClientReferenceApplyConfiguration{
				OIDCProviderName: &providerName,
				IssuerURL:        &providerIssuerURL,
				ClientID:         &c.currentClientID,
			},
		)
	}

	if authnConfig.Spec.Type == configv1.AuthenticationTypeOIDC {
		for _, conditionType := range []string{conditionTypeDegraded, conditionTypeProgressing, conditionTypeAvailable} {
			condition := c.conditionsToApply[conditionType]
			if condition == nil {
				condition = existingOrNewCondition(applyConfig, conditionType)
			}
			clientStatus.WithConditions(*condition)
		}
	}

	if applyConfig.Status != nil && equality.Semantic.DeepEqual(applyConfig.Status.OIDCClients, clientStatus) {
		return nil
	}

	updatedStatus := applyConfig.WithStatus(
		(&configv1ac.AuthenticationStatusApplyConfiguration{}).WithOIDCClients(clientStatus),
	)
	_, err = c.client.ApplyStatus(ctx, updatedStatus, metav1.ApplyOptions{FieldManager: c.fieldManager, Force: true})
	return err
}

func existingOrNewCondition(applyConfig *configv1ac.AuthenticationApplyConfiguration, conditionType string) *metav1.Condition {
	var condition *metav1.Condition
	if applyConfig.Status != nil && len(applyConfig.Status.OIDCClients) > 0 {
		slices.IndexFunc[metav1.Condition](applyConfig.Status.OIDCClients[0].Conditions, func(cond metav1.Condition) bool {
			if cond.Type == conditionType {
				condition = &cond
				return true
			}
			return false
		})
	}

	if condition == nil {
		condition = &metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
			Reason:             "Unknown",
		}
	}

	return condition
}
