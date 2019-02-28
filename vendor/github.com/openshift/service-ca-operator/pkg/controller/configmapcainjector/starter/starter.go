package starter

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-ca-operator/pkg/controller/configmapcainjector/controller"
)

func StartConfigMapCABundleInjector(ctx *controllercmd.ControllerContext) error {
	config := &scsv1alpha1.ConfigMapCABundleInjectorConfig{}
	if ctx.ComponentConfig != nil {
		// make a copy we can mutate
		configCopy := ctx.ComponentConfig.DeepCopy()
		// force the config to our version to read it
		configCopy.SetGroupVersionKind(scsv1alpha1.GroupVersion.WithKind("ConfigMapCABundleInjectorConfig"))
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(configCopy.Object, config); err != nil {
			return err
		}
	}
	if len(config.CABundleFile) == 0 {
		return fmt.Errorf("no ca bundle provided")
	}
	ca, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return err
	}
	// Verify that there is at least one cert in the bundle file
	block, _ := pem.Decode(ca)
	if block == nil {
		return fmt.Errorf("failed to parse CA bundle file as pem")
	}
	if _, err = x509.ParseCertificate(block.Bytes); err != nil {
		return err
	}
	caBundle := string(ca)

	kubeClient, err := kubernetes.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 20*time.Minute)

	configMapInjectorController := controller.NewConfigMapCABundleInjectionController(
		kubeInformers.Core().V1().ConfigMaps(),
		kubeClient.CoreV1(),
		caBundle,
	)

	kubeInformers.Start(ctx.Done())

	go configMapInjectorController.Run(5, ctx.Done())

	<-ctx.Done()

	return fmt.Errorf("stopped")
}
