package crdconversionwebhook

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	converter "github.com/openshift/console-operator/pkg/cmd/crdconversionwebhook/converter"
)

var (
	certFile string
	keyFile  string
	port     int
)

func NewConverter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crdconvert",
		Short: "Start server for CRD conversion",
		Run: func(command *cobra.Command, args []string) {
			startServer()
		},
	}
	cmd.Flags().StringVar(&certFile, "tls-cert-file", "", "File containing the default x509 Certificate for HTTPS.")
	cmd.Flags().StringVar(&keyFile, "tls-private-key-file", "", "File containing the default x509 private key matching --tls-cert-file.")
	cmd.Flags().IntVar(&port, "port", 443, "Secure port that the webhook listens on")

	return cmd
}

// Config contains the server (the webhook) cert and key.
type Config struct {
	CertFile string
	KeyFile  string
}

func startServer() {
	config := Config{CertFile: certFile, KeyFile: keyFile}

	http.HandleFunc("/crdconvert", converter.ServeExampleConvert)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	clientset := getClient()
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		TLSConfig:    configTLS(config, clientset),
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // disable HTTP/2
	}
	server.ListenAndServeTLS("", "")
}
