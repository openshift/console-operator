// go test github.com/openshift/console-operator/pkg/crypto
package crypto

import (
	"fmt"
	"testing"
)

func TestFakeThing(t *testing.T) {
	fmt.Println("test something!")
	if false {
		t.Errorf("The fake test said false, so that is bad.  :(")
	}
	// yup, runs
	//if true {
	//	t.Errorf("Bah! its true! woe is me")
	//}
}

// quick and dirty, how to test in golang:
// https://blog.alexellis.io/golang-writing-unit-tests/
//
//
// test at least one of these just to kick off tests quick:
// https://github.com/openshift/origin/blob/9a8a2fb3f9485bf88ebea61e4b5d8bf04dd3c459/pkg/oauthserver/server/crypto/random.go
// a quick test example:
// https://github.com/openshift/origin/blob/master/pkg/oc/cli/idle/idle_test.go
// call t.Error, t.Fail, t.Errorf to provide details of test failures
// call t.Log to provide non-failing debug information
// files saved as thing_test.go
func TestRandomBits(t *testing.T) {
	for i := 1; i <= 10; i++ {
		bits := RandomBitsString(256)
		fmt.Printf("The bits: %v \n", bits)
		if false {
			t.Errorf("Welp, it was false.")
		}
		// yup, runs
		//else {
		//	t.Errorf("True, oh the pain... %v", bits)
		//}
	}
}

// TestRandomBitsString
// TestRandom256BitsString
