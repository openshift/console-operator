package apiserver

import (
	"fmt"
	"reflect"
	"slices"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

type APIServerLister interface {
	APIServerLister() configlistersv1.APIServerLister
}

// ObserveTLSSecurityProfile observes APIServer.Spec.TLSSecurityProfile and writes
// servingInfo.minTLSVersion, servingInfo.cipherSuites, and servingInfo.groups.
func ObserveTLSSecurityProfile(listers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(
		listers,
		recorder,
		existingConfig,
		[]string{"servingInfo", "minTLSVersion"},
		[]string{"servingInfo", "cipherSuites"},
		[]string{"servingInfo", "groups"},
	)
}

func innerTLSSecurityProfileObservations(
	listers configobserver.Listers,
	recorder events.Recorder,
	existingConfig map[string]interface{},
	minTLSVersionPath, cipherSuitesPath, groupsPath []string,
) (ret map[string]interface{}, _ []error) {
	defer func() {
		ret = configobserver.Pruned(ret, minTLSVersionPath, cipherSuitesPath, groupsPath)
	}()

	apiServerListers := listers.(APIServerLister)
	errList := []error{}

	currentMinTLSVersion, _, versionErr := unstructured.NestedString(existingConfig, minTLSVersionPath...)
	if versionErr != nil {
		errList = append(errList, fmt.Errorf("failed to retrieve spec.servingInfo.minTLSVersion: %v", versionErr))
	}

	currentCipherSuites, _, suitesErr := unstructured.NestedStringSlice(existingConfig, cipherSuitesPath...)
	if suitesErr != nil {
		errList = append(errList, fmt.Errorf("failed to retrieve spec.servingInfo.cipherSuites: %v", suitesErr))
	}

	currentGroups, _, groupsErr := unstructured.NestedStringSlice(existingConfig, groupsPath...)
	if groupsErr != nil {
		errList = append(errList, fmt.Errorf("failed to retrieve spec.servingInfo.groups: %v", groupsErr))
	}

	apiServer, err := apiServerListers.APIServerLister().Get("cluster")
	if apierrors.IsNotFound(err) {
		klog.Warningf("apiserver.config.openshift.io/cluster: not found")
		apiServer = &configv1.APIServer{}
	} else if err != nil {
		return existingConfig, append(errList, err)
	}

	observedMinTLSVersion, observedCipherSuites, observedGroups := getSecurityProfileSettings(apiServer.Spec.TLSSecurityProfile)
	observedConfig := map[string]interface{}{}

	if err = unstructured.SetNestedField(observedConfig, observedMinTLSVersion, minTLSVersionPath...); err != nil {
		return existingConfig, append(errList, err)
	}
	if err = unstructured.SetNestedStringSlice(observedConfig, observedCipherSuites, cipherSuitesPath...); err != nil {
		return existingConfig, append(errList, err)
	}
	if len(observedGroups) > 0 {
		if err = unstructured.SetNestedStringSlice(observedConfig, observedGroups, groupsPath...); err != nil {
			return existingConfig, append(errList, err)
		}
	}

	if observedMinTLSVersion != currentMinTLSVersion {
		recorder.Eventf("ObserveTLSSecurityProfile", "minTLSVersion changed to %s", observedMinTLSVersion)
	}
	if !reflect.DeepEqual(observedCipherSuites, currentCipherSuites) {
		recorder.Eventf("ObserveTLSSecurityProfile", "cipherSuites changed to %q", observedCipherSuites)
	}
	if !slices.Equal(observedGroups, currentGroups) {
		recorder.Eventf("ObserveTLSSecurityProfile", "groups changed to %q", observedGroups)
	}

	return observedConfig, errList
}

func getSecurityProfileSettings(profile *configv1.TLSSecurityProfile) (string, []string, []string) {
	profileType := crypto.DefaultTLSProfileType
	if profile != nil {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[crypto.DefaultTLSProfileType]
	}

	srcGroups := profileSpec.Groups
	if isFIPSEnabled() {
		srcGroups = fipsApprovedTLSGroups(srcGroups)
	}

	groups := make([]string, 0, len(srcGroups))
	for _, group := range srcGroups {
		groups = append(groups, string(group))
	}

	return string(profileSpec.MinTLSVersion), crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers), groups
}
