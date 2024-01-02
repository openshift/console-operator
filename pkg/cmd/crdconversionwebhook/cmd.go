package crdconversionwebhook

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

func startServer() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize klog
	klog.InitFlags(nil)
	log.SetLogger(klog.NewKlogr()) // controller-runtime/log will complain if we don't set this

	// Log flag values for debugging
	klog.Infof("Starting console conversion webhook server")
	klog.V(4).Infof("Using flags:\n\t--tls-cert-file %s\n\t--tls-private-key-file %s\n\t--port %d", certFile, keyFile, port)

	// Initialize a new cert watcher with cert/key pair
	klog.V(4).Infof("Creating cert watcher")
	watcher, err := certwatcher.New(certFile, keyFile)
	if err != nil {
		klog.Fatalf("Error creating cert watcher: %v", err)
	}

	// Start goroutine with certwatcher running fsnotify against supplied certdir
	go func() {
		klog.V(4).Infof("Starting cert watcher")
		if err := watcher.Start(ctx); err != nil {
			klog.Fatalf("Cert watcher failed: %v", err)
		}
	}()

	// Setup TLS config using GetCertficate for fetching the cert when it changes
	tlsConfig := &tls.Config{
		GetCertificate: watcher.GetCertificate,
		NextProtos:     []string{"http/1.1"}, // Disable HTTP/2
	}

	// Create TLS listener
	klog.V(4).Infof("Creating TLS listener on port %d", port)
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	if err != nil {
		klog.Fatalf("Error creating TLS listener: %v", err)
	}

	// Setup handlers and server
	http.HandleFunc("/crdconvert", converter.ServeConsolePluginConvert)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	server := &http.Server{}

	// Shutdown server on context cancellation
	go func() {
		<-ctx.Done()
		klog.V(4).Info("Shutting down server")
		if err := server.Shutdown(context.Background()); err != nil {
			klog.Fatalf("Error shutting down server: %v", err)
		}
	}()

	// Serve
	klog.Infof("Serving on %s", listener.Addr().String())
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Error serving: %v", err)
	}
}
