package deployment

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

func TestNetworkPolicyAllowRulesMatchWorkloadManifests(t *testing.T) {
	consoleDeployment := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/deployments/console-deployment.yaml"))
	downloadsDeployment := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/deployments/downloads-deployment.yaml"))
	consoleService := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/services/console-service.yaml"))
	consoleRedirectService := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/services/console-redirect-service.yaml"))
	downloadsService := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/services/downloads-service.yaml"))

	tests := []struct {
		name                string
		policyFile          string
		deployment          *appsv1.Deployment
		services            []*corev1.Service
		ingressRuleServices [][]*corev1.Service
	}{
		{
			name:       "console UI egress",
			policyFile: "03-networkpolicy-console-allow-egress-console-ui.yaml",
			deployment: consoleDeployment,
			services:   []*corev1.Service{consoleService, consoleRedirectService},
		},
		{
			name:       "console UI ingress",
			policyFile: "03-networkpolicy-console-allow-ingress-console-ui.yaml",
			deployment: consoleDeployment,
			services:   []*corev1.Service{consoleService, consoleRedirectService},
			ingressRuleServices: [][]*corev1.Service{
				{consoleService, consoleRedirectService},
				{consoleService},
			},
		},
		{
			name:       "downloads ingress",
			policyFile: "03-networkpolicy-console-allow-ingress-downloads.yaml",
			deployment: downloadsDeployment,
			services:   []*corev1.Service{downloadsService},
			ingressRuleServices: [][]*corev1.Service{
				{downloadsService},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := readNetworkPolicy(t, tt.policyFile)

			assertNetworkPolicySelectorMatchesWorkload(t, policy, tt.deployment, tt.services)

			if len(policy.Spec.Ingress) != len(tt.ingressRuleServices) {
				t.Fatalf("expected %d ingress rules, got %d", len(tt.ingressRuleServices), len(policy.Spec.Ingress))
			}

			for index, services := range tt.ingressRuleServices {
				assertNetworkPolicyPortsMatchServiceTargets(t, policy.Spec.Ingress[index].Ports, services)
			}
		})
	}
}

func readNetworkPolicy(t *testing.T, policyFile string) *networkingv1.NetworkPolicy {
	t.Helper()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}

	policyBytes, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "../../../../manifests", policyFile))
	if err != nil {
		t.Fatalf("read NetworkPolicy manifest %q: %v", policyFile, err)
	}

	return resourceread.ReadNetworkPolicyV1OrDie(policyBytes)
}

func assertNetworkPolicySelectorMatchesWorkload(t *testing.T, policy *networkingv1.NetworkPolicy, deployment *appsv1.Deployment, services []*corev1.Service) {
	t.Helper()

	if diff := deep.Equal(deployment.Spec.Template.Labels, policy.Spec.PodSelector.MatchLabels); diff != nil {
		t.Errorf("NetworkPolicy selector does not match deployment template labels: %v", diff)
	}
	if diff := deep.Equal(deployment.Spec.Selector.MatchLabels, policy.Spec.PodSelector.MatchLabels); diff != nil {
		t.Errorf("NetworkPolicy selector does not match deployment selector: %v", diff)
	}

	for _, service := range services {
		if diff := deep.Equal(service.Spec.Selector, policy.Spec.PodSelector.MatchLabels); diff != nil {
			t.Errorf("NetworkPolicy selector does not match service %q selector: %v", service.Name, diff)
		}
	}
}

func assertNetworkPolicyPortsMatchServiceTargets(t *testing.T, policyPorts []networkingv1.NetworkPolicyPort, services []*corev1.Service) {
	t.Helper()

	actualPorts := map[intstr.IntOrString]int{}
	for _, policyPort := range policyPorts {
		if policyPort.Protocol == nil || *policyPort.Protocol != corev1.ProtocolTCP {
			t.Errorf("expected NetworkPolicy port protocol TCP, got %v", policyPort.Protocol)
		}
		if policyPort.Port == nil {
			t.Error("expected NetworkPolicy port to be set")
			continue
		}
		actualPorts[*policyPort.Port]++
	}

	expectedPorts := map[intstr.IntOrString]int{}
	for _, service := range services {
		for _, servicePort := range service.Spec.Ports {
			if servicePort.Protocol != corev1.ProtocolTCP {
				t.Errorf("expected service %q port %q protocol TCP, got %q", service.Name, servicePort.Name, servicePort.Protocol)
			}
			expectedPorts[servicePort.TargetPort]++
		}
	}

	if diff := deep.Equal(expectedPorts, actualPorts); diff != nil {
		t.Errorf("NetworkPolicy ports do not match service target ports: %v", diff)
	}
}
