package starter

import (
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-ca-operator/pkg/controller/apiservicecabundle/controller"
)

func StartAPIServiceCABundleInjector(ctx *controllercmd.ControllerContext) error {

	config := &scsv1alpha1.APIServiceCABundleInjectorConfig{}
	if ctx.ComponentConfig != nil {
		// make a copy we can mutate
		configCopy := ctx.ComponentConfig.DeepCopy()
		// force the config to our version to read it
		configCopy.SetGroupVersionKind(scsv1alpha1.GroupVersion.WithKind("APIServiceCABundleInjectorConfig"))
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(configCopy.Object, config); err != nil {
			return err
		}
	}

	if len(config.CABundleFile) == 0 {
		return fmt.Errorf("no signing cert/key pair provided")
	}

	caBundleContent, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return err
	}

	apiServiceClient, err := apiserviceclient.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}
	apiServiceInformers := apiserviceinformer.NewSharedInformerFactory(apiServiceClient, 2*time.Minute)

	servingCertUpdateController := controller.NewAPIServiceCABundleInjector(
		apiServiceInformers.Apiregistration().V1().APIServices(),
		apiServiceClient.ApiregistrationV1(),
		caBundleContent,
	)

	apiServiceInformers.Start(ctx.Done())

	go servingCertUpdateController.Run(5, ctx.Done())

	<-ctx.Done()

	return fmt.Errorf("stopped")
}
