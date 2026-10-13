package apiserver

import (
	"errors"
	"os"
	"testing"
)

func TestDetectFIPS(t *testing.T) {
	origLookupEnv := lookupEnv
	origReadFile := readFile
	defer func() {
		lookupEnv = origLookupEnv
		readFile = origReadFile
	}()

	tests := []struct {
		name         string
		envValue     string
		envPresent   bool
		readFileData []byte
		readFileErr  error
		want         bool
	}{
		{
			name:       "uses FIPS_ENABLED=true when present",
			envValue:   "true",
			envPresent: true,
			want:       true,
		},
		{
			name:       "uses FIPS_ENABLED=false when present",
			envValue:   "false",
			envPresent: true,
			want:       false,
		},
		{
			name:         "falls back to procfs when env invalid",
			envValue:     "not-a-bool",
			envPresent:   true,
			readFileData: []byte("1\n"),
			want:         true,
		},
		{
			name:         "reads procfs when env absent",
			readFileData: []byte("1\n"),
			want:         true,
		},
		{
			name:         "procfs zero means false",
			readFileData: []byte("0\n"),
			want:         false,
		},
		{
			name:        "read error means false",
			readFileErr: errors.New("boom"),
			want:        false,
		},
		{
			name:         "empty procfs means false",
			readFileData: []byte{},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupEnv = func(key string) (string, bool) {
				if key != "FIPS_ENABLED" {
					t.Fatalf("unexpected env lookup: %s", key)
				}
				return tt.envValue, tt.envPresent
			}
			readFile = func(name string) ([]byte, error) {
				if name != "/proc/sys/crypto/fips_enabled" {
					t.Fatalf("unexpected read path: %s", name)
				}
				return tt.readFileData, tt.readFileErr
			}

			if got := detectFIPS(); got != tt.want {
				t.Fatalf("detectFIPS() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestDetectFIPSWithRealEnvShape(t *testing.T) {
	origLookupEnv := lookupEnv
	origReadFile := readFile
	defer func() {
		lookupEnv = origLookupEnv
		readFile = origReadFile
	}()

	t.Setenv("FIPS_ENABLED", "true")
	lookupEnv = os.LookupEnv
	readFile = func(name string) ([]byte, error) {
		if name != "/proc/sys/crypto/fips_enabled" {
			t.Fatalf("unexpected read path: %s", name)
		}
		return []byte("0\n"), nil
	}

	if !detectFIPS() {
		t.Fatal("detectFIPS() = false, want true")
	}
}
