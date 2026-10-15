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
// servingInfo.minTLSVersion, servingInfo.cipherSuites, and servingInfo.tlsGroups.
func ObserveTLSSecurityProfile(listers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(
		listers,
		recorder,
		existingConfig,
		[]string{"servingInfo", "minTLSVersion"},
		[]string{"servingInfo", "cipherSuites"},
		[]string{"servingInfo", "tlsGroups"},
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
		errList = append(errList, fmt.Errorf("failed to read servingInfo.minTLSVersion from the existing observed config: %w; inspect spec.observedConfig on console.operator.openshift.io/cluster for malformed values", versionErr))
	}

	currentCipherSuites, _, suitesErr := unstructured.NestedStringSlice(existingConfig, cipherSuitesPath...)
	if suitesErr != nil {
		errList = append(errList, fmt.Errorf("failed to read servingInfo.cipherSuites from the existing observed config: %w; inspect spec.observedConfig on console.operator.openshift.io/cluster for malformed values", suitesErr))
	}

	currentGroups, _, groupsErr := unstructured.NestedStringSlice(existingConfig, groupsPath...)
	if groupsErr != nil {
		errList = append(errList, fmt.Errorf("failed to read servingInfo.tlsGroups from the existing observed config: %w; inspect spec.observedConfig on console.operator.openshift.io/cluster for malformed values", groupsErr))
	}

	apiServer, err := apiServerListers.APIServerLister().Get("cluster")
	if apierrors.IsNotFound(err) {
		klog.Warningf("apiserver.config.openshift.io/cluster not found; applying the default TLS security profile (%s). Create the APIServer config resource to customize TLS settings.", crypto.DefaultTLSProfileType)
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
		recorder.Eventf("ObserveTLSSecurityProfile", "minTLSVersion changed from %q to %q", currentMinTLSVersion, observedMinTLSVersion)
	}
	if !reflect.DeepEqual(observedCipherSuites, currentCipherSuites) {
		recorder.Eventf("ObserveTLSSecurityProfile", "cipherSuites changed from %q to %q", currentCipherSuites, observedCipherSuites)
	}
	if !slices.Equal(observedGroups, currentGroups) {
		recorder.Eventf("ObserveTLSSecurityProfile", "tlsGroups changed from %q to %q", currentGroups, observedGroups)
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
	if isFIPSEnabled() && len(srcGroups) > 0 {
		approved := fipsApprovedTLSGroups(srcGroups)
		if len(approved) != len(srcGroups) {
			klog.Warningf("FIPS mode is enabled; restricting the configured TLS groups %v to the FIPS-approved set %v. Non-approved groups are ignored and the console falls back to the Go runtime default FIPS curves when the set is empty. To avoid this, configure only FIPS-approved NIST curves (secp256r1, secp384r1, secp521r1) in the APIServer TLSSecurityProfile.", srcGroups, approved)
		}
		srcGroups = approved
	}

	groups := make([]string, 0, len(srcGroups))
	for _, group := range srcGroups {
		groups = append(groups, string(group))
	}

	return string(profileSpec.MinTLSVersion), crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers), groups
}
