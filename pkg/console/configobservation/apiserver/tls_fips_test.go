package apiserver

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestFIPSApprovedTLSGroups(t *testing.T) {
	groups := []configv1.TLSGroup{
		configv1.TLSGroupX25519MLKEM768,
		configv1.TLSGroupX25519,
		configv1.TLSGroupSecP256r1,
		configv1.TLSGroupSecP384r1,
		configv1.TLSGroupSecP521r1,
	}

	got := fipsApprovedTLSGroups(groups)
	want := []configv1.TLSGroup{
		configv1.TLSGroupSecP256r1,
		configv1.TLSGroupSecP384r1,
		configv1.TLSGroupSecP521r1,
	}

	if len(got) != len(want) {
		t.Fatalf("fipsApprovedTLSGroups() len = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fipsApprovedTLSGroups()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
