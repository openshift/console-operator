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

// writeJSON writes a JSON response and reports errors through t.
func writeJSON(t *testing.T, w http.ResponseWriter, format string, args ...interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}

// certPEM returns the PEM-encoded certificate of a TLS test server.
func certPEM(s *httptest.Server) []byte {
	cert := s.TLS.Certificates[0]
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})
}

func TestValidateOIDCIssuer(t *testing.T) {
	// Create a TLS test server that echoes back its own URL as the issuer
	var validServer *httptest.Server
	validServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			writeJSON(t, w, `{"issuer": %q}`, validServer.URL)
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

	// Server that returns HTML instead of JSON
	htmlServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := fmt.Fprint(w, "<html>not json</html>"); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer htmlServer.Close()
	htmlServerCAPEM := certPEM(htmlServer)

	// Server that returns JSON with a mismatched issuer
	mismatchServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"issuer": "https://wrong-issuer.example.com"}`)
	}))
	defer mismatchServer.Close()
	mismatchServerCAPEM := certPEM(mismatchServer)

	// Server that returns invalid JSON
	invalidJSONServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{invalid json}`)
	}))
	defer invalidJSONServer.Close()
	invalidJSONServerCAPEM := certPEM(invalidJSONServer)

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
		{
			name:      "URL with query component",
			issuerURL: "https://example.com?foo=bar",
			wantErr:   true,
			errSubstr: "must not contain a query",
		},
		{
			name:      "URL with fragment component",
			issuerURL: "https://example.com#frag",
			wantErr:   true,
			errSubstr: "must not contain a fragment",
		},
		{
			name:      "discovery returns non-JSON content type",
			issuerURL: htmlServer.URL,
			caBundle:  htmlServerCAPEM,
			wantErr:   true,
			errSubstr: "non-JSON content type",
		},
		{
			name:      "discovery issuer mismatch",
			issuerURL: mismatchServer.URL,
			caBundle:  mismatchServerCAPEM,
			wantErr:   true,
			errSubstr: "does not match configured issuer",
		},
		{
			name:      "discovery returns invalid JSON",
			issuerURL: invalidJSONServer.URL,
			caBundle:  invalidJSONServerCAPEM,
			wantErr:   true,
			errSubstr: "not valid JSON",
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
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"issuer": %q}`, server.URL)
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
				MinVersion: tls.VersionTLS12,
				RootCAs:    pool,
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to reach test server: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
