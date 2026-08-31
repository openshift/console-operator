package oidcsetup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// certPEM returns the PEM-encoded certificate of a TLS test server.
func certPEM(s *httptest.Server) []byte {
	cert := s.TLS.Certificates[0]
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})
}

func TestValidateOIDCIssuer(t *testing.T) {
	// Create a TLS test server that serves a valid OIDC discovery response
	validServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"issuer": "%s"}`, "https://valid-issuer")
			return
		}
		http.NotFound(w, r)
	}))
	defer validServer.Close()

	validServerCAPEM := certPEM(validServer)

	// Server that returns 404 for discovery
	notFoundServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer notFoundServer.Close()
	notFoundServerCAPEM := certPEM(notFoundServer)

	// Server that returns 500
	errorServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()
	errorServerCAPEM := certPEM(errorServer)

	tests := []struct {
		name      string
		issuerURL string
		caBundle  []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "empty URL",
			issuerURL: "",
			wantErr:   true,
			errSubstr: "issuer URL is empty",
		},
		{
			name:      "non-HTTPS scheme",
			issuerURL: "http://example.com",
			wantErr:   true,
			errSubstr: "must use HTTPS",
		},
		{
			name:      "missing host",
			issuerURL: "https://",
			wantErr:   true,
			errSubstr: "has no host",
		},
		{
			name:      "malformed URL",
			issuerURL: "://bad-url",
			wantErr:   true,
			errSubstr: "malformed",
		},
		{
			name:      "valid OIDC discovery",
			issuerURL: validServer.URL,
			caBundle:  validServerCAPEM,
			wantErr:   false,
		},
		{
			name:      "valid OIDC discovery with trailing slash",
			issuerURL: validServer.URL + "/",
			caBundle:  validServerCAPEM,
			wantErr:   false,
		},
		{
			name:      "discovery returns 404",
			issuerURL: notFoundServer.URL,
			caBundle:  notFoundServerCAPEM,
			wantErr:   true,
			errSubstr: "HTTP 404",
		},
		{
			name:      "discovery returns 500",
			issuerURL: errorServer.URL,
			caBundle:  errorServerCAPEM,
			wantErr:   true,
			errSubstr: "HTTP 500",
		},
		{
			name:      "unreachable host",
			issuerURL: "https://192.0.2.1:1",
			wantErr:   true,
			errSubstr: "not reachable",
		},
		{
			name:      "custom CA bundle succeeds",
			issuerURL: validServer.URL,
			caBundle:  validServerCAPEM,
			wantErr:   false,
		},
		{
			name:      "missing CA bundle for self-signed cert fails",
			issuerURL: validServer.URL,
			caBundle:  nil,
			wantErr:   true,
			errSubstr: "not reachable",
		},
		{
			name:      "invalid CA bundle PEM",
			issuerURL: validServer.URL,
			caBundle:  []byte("not-a-valid-pem"),
			wantErr:   true,
			errSubstr: "failed to parse CA bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := validateOIDCIssuer(ctx, tt.issuerURL, tt.caBundle)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateOIDCIssuerTLSConfig verifies that TLS configuration
// with a custom CA bundle works correctly end-to-end.
func TestValidateOIDCIssuerTLSConfig(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"issuer": "https://test"}`)
	}))
	defer server.Close()

	caBundle := certPEM(server)

	// Verify we can reach the server with the correct CA
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBundle) {
		t.Fatal("failed to add server cert to pool")
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	resp, err := client.Get(server.URL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("failed to reach test server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
