package oidcsetup

import (
	"context"
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
		if r.URL.Path == "/.well-known/openid-configuration" || r.URL.Path == "/custom-discovery" {
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

	// Server that returns valid JSON but without a Content-Type header
	var noContentTypeServer *httptest.Server
	noContentTypeServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove auto-detected Content-Type before writing the response.
		// Setting the map entry to nil prevents Go from sniffing the type
		// while ensuring no Content-Type header is sent on the wire.
		w.Header()["Content-Type"] = nil
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, `{"issuer": %q}`, noContentTypeServer.URL); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer noContentTypeServer.Close()
	noContentTypeServerCAPEM := certPEM(noContentTypeServer)

	// Server that only serves discovery at a custom path (not .well-known),
	// simulating a provider that requires discoveryURL override.
	var customDiscoveryServer *httptest.Server
	customDiscoveryServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oidc/.well-known/openid-configuration" {
			writeJSON(t, w, `{"issuer": %q}`, customDiscoveryServer.URL)
			return
		}
		http.NotFound(w, r)
	}))
	defer customDiscoveryServer.Close()
	customDiscoveryServerCAPEM := certPEM(customDiscoveryServer)

	tests := []struct {
		name         string
		issuerURL    string
		discoveryURL string
		caBundle     []byte
		wantErr      bool
		errSubstr    string
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
		{
			name:      "discovery returns no Content-Type header",
			issuerURL: noContentTypeServer.URL,
			caBundle:  noContentTypeServerCAPEM,
			wantErr:   true,
			errSubstr: "no Content-Type header",
		},
		{
			name:         "valid with custom discoveryURL",
			issuerURL:    validServer.URL,
			discoveryURL: validServer.URL + "/custom-discovery",
			caBundle:     validServerCAPEM,
			wantErr:      false,
		},
		{
			name:         "custom discoveryURL overrides default path",
			issuerURL:    customDiscoveryServer.URL,
			discoveryURL: customDiscoveryServer.URL + "/oidc/.well-known/openid-configuration",
			caBundle:     customDiscoveryServerCAPEM,
			wantErr:      false,
		},
		{
			name:      "discoveryURL not set falls back to default path which 404s",
			issuerURL: customDiscoveryServer.URL,
			caBundle:  customDiscoveryServerCAPEM,
			wantErr:   true,
			errSubstr: "HTTP 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := validateOIDCIssuer(ctx, tt.issuerURL, tt.discoveryURL, tt.caBundle)

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

// TestValidateOIDCIssuerTLSConfig exercises validateOIDCIssuer with TLS
// servers to verify that the production TLS configuration (MinVersion 1.2,
// custom CA bundle) works end-to-end.
func TestValidateOIDCIssuerTLSConfig(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"issuer": %q}`, server.URL)
	}))
	defer server.Close()

	caBundle := certPEM(server)
	ctx := context.Background()

	// TLS 1.2+ server with correct CA should succeed
	if err := validateOIDCIssuer(ctx, server.URL, "", caBundle); err != nil {
		t.Fatalf("expected success with correct CA bundle, got: %v", err)
	}

	// Without CA bundle, TLS verification should fail
	if err := validateOIDCIssuer(ctx, server.URL, "", nil); err == nil {
		t.Fatal("expected TLS verification error without CA bundle, got nil")
	}
}
