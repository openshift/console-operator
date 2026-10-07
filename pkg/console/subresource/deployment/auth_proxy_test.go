package deployment

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/proxyconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthProxyDeployment(t *testing.T) {
	build := func(proxy *proxyconfig.Config) *appsv1.Deployment {
		return DefaultDeployment(&operatorv1.Console{},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "console-config"}},
			&corev1.ConfigMap{}, nil,
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "issuer-ca"}},
			&corev1.ConfigMap{Data: map[string]string{api.TrustedCABundleKey: "global-trust"}},
			&corev1.Secret{}, &corev1.Secret{}, &corev1.Secret{},
			&configv1.Proxy{Status: configv1.ProxyStatus{HTTPProxy: "http://global.example:3128", HTTPSProxy: "http://global.example:3129", NoProxy: "unchanged.example, unchanged.example"}},
			&configv1.Infrastructure{Status: configv1.InfrastructureStatus{ControlPlaneTopology: configv1.HighlyAvailableTopologyMode}}, proxy)
	}
	baseline := build(nil)
	withoutCA := build(&proxyconfig.Config{HTTPSProxy: "http://component.example:3128"})
	if !reflect.DeepEqual(baseline.Spec.Template, withoutCA.Spec.Template) {
		t.Fatal("routing settings changed environment, volumes, or trust outside Console configuration")
	}
	proxy := &proxyconfig.Config{HTTPSProxy: "http://component.example:3128", TrustedCAName: "proxy-ca"}
	withCA := build(proxy)
	container := withCA.Spec.Template.Spec.Containers[0]
	if !reflect.DeepEqual(container.Env, baseline.Spec.Template.Spec.Containers[0].Env) {
		t.Fatal("OIDC proxy changed global proxy environment")
	}
	var mount *corev1.VolumeMount
	for i := range container.VolumeMounts {
		if container.VolumeMounts[i].Name == api.AuthProxyCAConfigMapName {
			mount = &container.VolumeMounts[i]
		}
	}
	if mount == nil || mount.MountPath != api.AuthProxyCAMountDir || !mount.ReadOnly || mount.SubPath != "" || mount.SubPathExpr != "" {
		t.Fatalf("unexpected proxy CA mount: %#v", mount)
	}
	var volume *corev1.Volume
	for i := range withCA.Spec.Template.Spec.Volumes {
		if withCA.Spec.Template.Spec.Volumes[i].Name == api.AuthProxyCAConfigMapName {
			volume = &withCA.Spec.Template.Spec.Volumes[i]
		}
	}
	if volume == nil || volume.ConfigMap == nil || volume.ConfigMap.Name != api.AuthProxyCAConfigMapName || !reflect.DeepEqual(volume.ConfigMap.Items, []corev1.KeyToPath{{Key: api.AuthProxyCAFileName, Path: api.AuthProxyCAFileName}}) {
		t.Fatalf("unexpected proxy CA projection: %#v", volume)
	}
	if withCA.Spec.Template.Annotations[authProxyCAConfigMapAnnotation] != "proxy-ca" {
		t.Fatal("CA source identity missing from Pod template")
	}
	// Removing the dedicated mount leaves every existing mount and volume intact.
	filtered := withCA.DeepCopy()
	delete(filtered.Spec.Template.Annotations, authProxyCAConfigMapAnnotation)
	filtered.Spec.Template.Spec.Volumes = nil
	for _, v := range withCA.Spec.Template.Spec.Volumes {
		if v.Name != api.AuthProxyCAConfigMapName {
			filtered.Spec.Template.Spec.Volumes = append(filtered.Spec.Template.Spec.Volumes, v)
		}
	}
	filtered.Spec.Template.Spec.Containers[0].VolumeMounts = nil
	for _, m := range container.VolumeMounts {
		if m.Name != api.AuthProxyCAConfigMapName {
			filtered.Spec.Template.Spec.Containers[0].VolumeMounts = append(filtered.Spec.Template.Spec.Containers[0].VolumeMounts, m)
		}
	}
	if !reflect.DeepEqual(filtered.Spec.Template, baseline.Spec.Template) {
		t.Fatal("dedicated proxy trust changed existing Pod configuration")
	}
	proxy.TrustedCAName = "replacement-ca"
	changed := build(proxy)
	if reflect.DeepEqual(changed.Spec.Template, withCA.Spec.Template) {
		t.Fatal("CA reference change did not change Pod template")
	}
	if !reflect.DeepEqual(build(nil).Spec.Template, baseline.Spec.Template) {
		t.Fatal("removal retained proxy mount or annotation")
	}
}
