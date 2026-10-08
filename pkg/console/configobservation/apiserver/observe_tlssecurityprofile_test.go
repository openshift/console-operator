package apiserver

import (
	"reflect"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/console-operator/pkg/console/configobservation"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	clocktesting "k8s.io/utils/clock/testing"
)

func newAPIServerListers(t *testing.T, apiServer *configv1.APIServer) configobservation.Listers {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if apiServer != nil {
		if err := indexer.Add(apiServer); err != nil {
			t.Fatalf("failed to add APIServer to indexer: %v", err)
		}
	}
	return configobservation.Listers{
		APIServerLister_: configlistersv1.NewAPIServerLister(indexer),
	}
}

func toStrings(groups []configv1.TLSGroup) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, string(g))
	}
	return out
}

func TestGetSecurityProfileSettings(t *testing.T) {
	intermediate := configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	modern := configv1.TLSProfiles[configv1.TLSProfileModernType]
	defaultProfile := configv1.TLSProfiles[crypto.DefaultTLSProfileType]

	tests := []struct {
		name        string
		profile     *configv1.TLSSecurityProfile
		wantVersion string
		wantGroups  []string
	}{
		{
			name:        "nil profile falls back to default profile",
			profile:     nil,
			wantVersion: string(defaultProfile.MinTLSVersion),
			wantGroups:  toStrings(defaultProfile.Groups),
		},
		{
			name:        "intermediate profile",
			profile:     &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			wantVersion: string(intermediate.MinTLSVersion),
			wantGroups:  toStrings(intermediate.Groups),
		},
		{
			name:        "modern profile",
			profile:     &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			wantVersion: string(modern.MinTLSVersion),
			wantGroups:  toStrings(modern.Groups),
		},
		{
			name: "custom profile with groups",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS13,
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
						Groups:        []configv1.TLSGroup{configv1.TLSGroupX25519, configv1.TLSGroupSecP256r1},
					},
				},
			},
			wantVersion: string(configv1.VersionTLS13),
			wantGroups:  []string{"X25519", "secp256r1"},
		},
		{
			name:        "custom profile type with nil custom spec falls back to default",
			profile:     &configv1.TLSSecurityProfile{Type: configv1.TLSProfileCustomType},
			wantVersion: string(defaultProfile.MinTLSVersion),
			wantGroups:  toStrings(defaultProfile.Groups),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Guard: this exercises the non-FIPS path. If the test binary were
			// ever run under FIPS the expected group sets would differ.
			if isFIPSEnabled() {
				t.Skip("skipping: expectations assume non-FIPS runtime")
			}

			gotVersion, _, gotGroups := getSecurityProfileSettings(tt.profile)
			if gotVersion != tt.wantVersion {
				t.Errorf("minTLSVersion = %q, want %q", gotVersion, tt.wantVersion)
			}
			if !reflect.DeepEqual(gotGroups, tt.wantGroups) {
				t.Errorf("groups = %v, want %v", gotGroups, tt.wantGroups)
			}
		})
	}
}

func TestObserveTLSSecurityProfile(t *testing.T) {
	recorder := events.NewInMemoryRecorder("test", clocktesting.NewFakePassiveClock(time.Now()))
	intermediate := configv1.TLSProfiles[configv1.TLSProfileIntermediateType]

	t.Run("writes groups from the configured profile", func(t *testing.T) {
		if isFIPSEnabled() {
			t.Skip("skipping: expectations assume non-FIPS runtime")
		}

		listers := newAPIServerListers(t, &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			},
		})

		observed, errs := ObserveTLSSecurityProfile(listers, recorder, map[string]interface{}{})
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}

		gotGroups, _, err := unstructured.NestedStringSlice(observed, "servingInfo", "groups")
		if err != nil {
			t.Fatalf("failed to read observed groups: %v", err)
		}
		if want := toStrings(intermediate.Groups); !reflect.DeepEqual(gotGroups, want) {
			t.Errorf("observed groups = %v, want %v", gotGroups, want)
		}

		gotVersion, _, err := unstructured.NestedString(observed, "servingInfo", "minTLSVersion")
		if err != nil {
			t.Fatalf("failed to read observed minTLSVersion: %v", err)
		}
		if gotVersion != string(intermediate.MinTLSVersion) {
			t.Errorf("observed minTLSVersion = %q, want %q", gotVersion, intermediate.MinTLSVersion)
		}
	})

	t.Run("missing apiserver falls back to default profile without error", func(t *testing.T) {
		listers := newAPIServerListers(t, nil)

		observed, errs := ObserveTLSSecurityProfile(listers, recorder, map[string]interface{}{})
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if _, _, err := unstructured.NestedString(observed, "servingInfo", "minTLSVersion"); err != nil {
			t.Fatalf("failed to read observed minTLSVersion: %v", err)
		}
	})

	t.Run("prunes fields outside the managed serving paths", func(t *testing.T) {
		listers := newAPIServerListers(t, &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			},
		})

		existing := map[string]interface{}{
			"servingInfo": map[string]interface{}{
				"unmanaged": "should-be-pruned",
			},
		}

		observed, errs := ObserveTLSSecurityProfile(listers, recorder, existing)
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if _, found, _ := unstructured.NestedString(observed, "servingInfo", "unmanaged"); found {
			t.Errorf("expected unmanaged field to be pruned from observed config")
		}
	})
}
