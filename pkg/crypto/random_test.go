package crypto

import (
	"fmt"
	"testing"
)

func TestFakeThing(t testing.T) {
	fmt.Println("test something!")
	if false {
		t.Errorf("The fake test said false, so that is bad.  :(")
	}
}

// test at least one of these just to kick off tests quick:
https://github.com/openshift/origin/blob/9a8a2fb3f9485bf88ebea61e4b5d8bf04dd3c459/pkg/oauthserver/server/crypto/random.go
// a quick test example:
// https://github.com/openshift/origin/blob/master/pkg/oc/cli/idle/idle_test.go
// TestRandomBits
// TestRandomBitsString
// TestRandom256BitsString
