package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	v1 "k8s.io/client-go/listers/core/v1"

	"github.com/openshift/console-operator/pkg/api"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

const (
	TelemeterClientDeploymentName      = "telemeter-client"
	TelemetryAnnotationPrefix          = "telemetry.console.openshift.io/"
	TelemeterClientDeploymentNamespace = "openshift-monitoring"
	PullSecretName                     = "pull-secret"
)

func IsTelemeterClientAvailable(deploymentLister appsv1listers.DeploymentLister) (bool, error) {
	deployment, err := deploymentLister.Deployments(TelemeterClientDeploymentNamespace).Get(TelemeterClientDeploymentName)

	if errors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return deploymentsub.IsAvailable(deployment), nil
}

func GetClusterID(clusterVersionLister configlistersv1.ClusterVersionLister) (string, error) {
	cv, cvErr := clusterVersionLister.Get(api.VersionResourceName)
	if cvErr != nil {
		return "", cvErr
	}
	return string(cv.Spec.ClusterID), nil
}

type DockerConfig struct {
	Auths map[string]DockerAuthEntry `json:"auths"`
}

// DockerAuthEntry contains the base64-encoded auth credentials for a Docker registry.
type DockerAuthEntry struct {
	Auth string `json:"auth"`
}

func GetAccessToken(secretsLister v1.SecretLister) (string, error) {
	secret, err := secretsLister.Secrets(api.OpenShiftConfigNamespace).Get(PullSecretName)
	if err != nil {
		return "", err
	}

	configBytes, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return "", fmt.Errorf("failed to parse .dockerconfigjson field from pull-secret")
	}
	var config DockerConfig
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return "", err
	}
	authsBytes, ok := config.Auths["cloud.openshift.com"]
	if !ok {
		return "", fmt.Errorf("failed to parse 'cloud.openshift.com' field from pull-secret")
	}
	return authsBytes.Auth, nil
}

// Needed to create our own types for OCM Subscriptions since their types and client are useless
// https://github.com/openshift-online/ocm-sdk-go/blob/main/accountsmgmt/v1/subscription_client.go - everything private
// https://github.com/openshift-online/ocm-sdk-go/blob/main/accountsmgmt/v1/subscriptions_client.go#L38-L41 - useless client
type OCMAPIResponse struct {
	Items []Organization `json:"items"`
}
type Organization struct {
	OrganizationID string `json:"organization_id"`
}

// GetOrganizationID fetches the organization ID using the cluster ID and access token
func GetOrganizationID(clusterID, accessToken string) (string, error) {
	u, err := buildURL(clusterID)
	if err != nil {
		return "", err // more contextual error handling can be added here if needed
	}

	req, err := createRequest(u, clusterID, accessToken)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to GET (%s): %v", u.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status '%s'", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var ocmResponse OCMAPIResponse
	if err = json.Unmarshal(body, &ocmResponse); err != nil {
		return "", fmt.Errorf("error decoding JSON: %v", err)
	}

	if len(ocmResponse.Items) == 0 {
		return "", fmt.Errorf("empty OCM response")
	}

	return ocmResponse.Items[0].OrganizationID, nil
}

// buildURL constructs the URL for the API request
func buildURL(clusterID string) (*url.URL, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "api.openshift.com",
		Path:   "api/accounts_mgmt/v1/subscriptions",
	}
	q := u.Query()
	q.Add("fetchOrganization", "true")
	q.Add("search", fmt.Sprintf("external_cluster_id='%s'", clusterID))
	u.RawQuery = q.Encode()
	return u, nil
}

// createRequest initializes the HTTP request with necessary headers
func createRequest(u *url.URL, clusterID, accessToken string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %v", err)
	}

	authHeader := fmt.Sprintf("AccessToken %s:%s", clusterID, accessToken)
	req.Header.Add("Authorization", authHeader)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	return req, nil
}
