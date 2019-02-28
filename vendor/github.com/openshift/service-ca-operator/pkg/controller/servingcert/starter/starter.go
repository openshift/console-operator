package starter

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-ca-operator/pkg/controller/servingcert/controller"
)

func StartServiceServingCertSigner(ctx *controllercmd.ControllerContext) error {

	config := &scsv1alpha1.ServiceServingCertSignerConfig{}
	if ctx.ComponentConfig != nil {
		// make a copy we can mutate
		configCopy := ctx.ComponentConfig.DeepCopy()
		// force the config to our version to read it
		configCopy.SetGroupVersionKind(scsv1alpha1.GroupVersion.WithKind("ServiceServingCertSignerConfig"))
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(configCopy.Object, config); err != nil {
			return err
		}
	}
	ca, err := crypto.GetCA(config.Signer.CertFile, config.Signer.KeyFile, "")
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 20*time.Minute)

	servingCertController := controller.NewServiceServingCertController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
	)
	servingCertUpdateController := controller.NewServiceServingCertUpdateController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
	)

	kubeInformers.Start(ctx.Done())

	go servingCertController.Run(5, ctx.Done())
	go servingCertUpdateController.Run(5, ctx.Done())

	<-ctx.Done()

	return fmt.Errorf("stopped")
}
