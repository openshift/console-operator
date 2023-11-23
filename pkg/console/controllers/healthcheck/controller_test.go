package healthcheck

import (
	"testing"

	"github.com/go-test/deep"
	configv1 "github.com/openshift/api/config/v1"
)

func TestGetPlatformURL(t *testing.T) {
	type args struct {
		ingressConfig        *configv1.Ingress
		infrastructureConfig *configv1.Infrastructure
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test NLB cluster setup",
			args: args{
				ingressConfig: &configv1.Ingress{
					Spec: configv1.IngressSpec{
						LoadBalancer: configv1.LoadBalancer{
							Platform: configv1.IngressPlatformSpec{
								Type: configv1.AWSPlatformType,
								AWS: &configv1.AWSIngressSpec{
									Type: configv1.NLB,
								},
							},
						},
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						PlatformStatus: &configv1.PlatformStatus{
							Type: configv1.AWSPlatformType,
						},
						ControlPlaneTopology: configv1.ExternalTopologyMode,
					},
				},
			},
			want: true,
		},
		{
			name: "Test non-NLB cluster setup with classic AWS LoadBalancer type",
			args: args{
				ingressConfig: &configv1.Ingress{
					Spec: configv1.IngressSpec{
						LoadBalancer: configv1.LoadBalancer{
							Platform: configv1.IngressPlatformSpec{
								Type: configv1.AWSPlatformType,
								AWS: &configv1.AWSIngressSpec{
									Type: configv1.Classic,
								},
							},
						},
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						PlatformStatus: &configv1.PlatformStatus{
							Type: configv1.AWSPlatformType,
						},
						ControlPlaneTopology: configv1.ExternalTopologyMode,
					},
				},
			},
			want: false,
		},
		{
			name: "Test non-NLB cluster setup with GCP LoadBalancer type",
			args: args{
				ingressConfig: &configv1.Ingress{
					Spec: configv1.IngressSpec{
						LoadBalancer: configv1.LoadBalancer{
							Platform: configv1.IngressPlatformSpec{
								Type: configv1.GCPPlatformType,
							},
						},
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						PlatformStatus: &configv1.PlatformStatus{
							Type: configv1.GCPPlatformType,
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(isExternalControlPlaneWithNLB(tt.args.infrastructureConfig, tt.args.ingressConfig), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
