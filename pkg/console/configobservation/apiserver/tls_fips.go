package apiserver

import (
	"crypto/fips140"

	configv1 "github.com/openshift/api/config/v1"
)

// fipsTLSGroups is a strict allowlist of the TLS named groups we apply when the
// operator is running in FIPS mode. Only the NIST elliptic curves are included;
// any group not listed here is dropped from the serving config under FIPS.
//
// This is deliberately conservative:
//   - X25519 is not a NIST curve and is therefore excluded.
//   - The ML-KEM hybrids (X25519MLKEM768, SecP256r1MLKEM768, SecP384r1MLKEM1024)
//     are excluded for now. ML-KEM itself is FIPS 203 approved, and the
//     NIST-curve hybrids (SecP256r1MLKEM768, SecP384r1MLKEM1024) are plausibly
//     FIPS-approvable, but they have not yet been verified for this config path,
//     so they default to non-FIPS until explicitly validated. The practical
//     consequence is that there is currently no PQC group available under FIPS;
//     revisit once the hybrid groups are verified (see CONSOLE-5477).
var fipsTLSGroups = map[configv1.TLSGroup]struct{}{
	configv1.TLSGroupSecP256r1: {},
	configv1.TLSGroupSecP384r1: {},
	configv1.TLSGroupSecP521r1: {},
}

func isFIPSEnabled() bool {
	return fips140.Enabled()
}

func fipsApprovedTLSGroups(groups []configv1.TLSGroup) []configv1.TLSGroup {
	approved := make([]configv1.TLSGroup, 0, len(groups))
	for _, group := range groups {
		if _, ok := fipsTLSGroups[group]; ok {
			approved = append(approved, group)
		}
	}
	return approved
}
