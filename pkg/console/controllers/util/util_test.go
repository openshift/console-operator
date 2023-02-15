package util

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterclientv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var (
	fooLabels = map[string]string{"foo": "foo", "baz": "baz"}
	barLabels = map[string]string{"bar": "bar"}
	fooObject = metav1.Object(
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo", Labels: fooLabels,
			},
		},
	)
	barObject = metav1.Object(
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar", Labels: barLabels,
			},
		},
	)
	validOpenShiftCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	validROSACluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "ROSA"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	validAROCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "ARO"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	validROKSCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "ROKS"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	validOpenShiftDedicatedCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShiftDedicated"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	invalidProductCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "EKS"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	emptyProductClaimCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: ""},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	missingProductClaimCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	availableFalseCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionFalse,
				},
			},
		},
	}
	availableUnknownCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionUnknown,
				},
			},
		},
	}
	availableMissingCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{},
		},
	}
	unsupportedVersionCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "3.11.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	emptyVersionClaimCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: ""},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	malformedVersionClaimCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "foobar"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	missingVersionClaimCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	emptyUrlCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "",
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	missingUrlCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					CABundle: []byte("foobarbaz"),
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	emptyCABundleCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      "https://foo.bar.baz",
					CABundle: []byte{},
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	missingCABundleCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL: "https://foo.bar.baz",
				},
			},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	emptyClientConfigCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{
			ManagedClusterClientConfigs: []clusterv1.ClientConfig{},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	missingClientConfigCluster = clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: api.ManagedClusterProductClaim, Value: "OpenShift"},
				{Name: api.ManagedClusterVersionClaim, Value: "4.0.0"},
			},
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1.ManagedClusterConditionAvailable,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	clusterList = clusterv1.ManagedClusterList{
		Items: []clusterv1.ManagedCluster{
			validOpenShiftCluster,
			validROSACluster,
			validAROCluster,
			validROKSCluster,
			validOpenShiftDedicatedCluster,
			invalidProductCluster,
			emptyProductClaimCluster,
			missingProductClaimCluster,
			availableFalseCluster,
			availableUnknownCluster,
			availableMissingCluster,
			unsupportedVersionCluster,
			malformedVersionClaimCluster,
			emptyVersionClaimCluster,
			missingVersionClaimCluster,
			emptyUrlCluster,
			missingUrlCluster,
			emptyCABundleCluster,
			missingCABundleCluster,
			emptyClientConfigCluster,
			missingClientConfigCluster,
		},
	}
	validClusterSlice = []clusterv1.ManagedCluster{
		validOpenShiftCluster,
		validROSACluster,
		validAROCluster,
		validROKSCluster,
		validOpenShiftDedicatedCluster,
	}
	notFoundError = &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Reason: metav1.StatusReasonNotFound,
			Code:   http.StatusNotFound,
		},
	}
	testError = errors.New("test")
)

// Mock a managed cluster client that returns a real list of managed clusters
type mockManagedClusterInterface struct {
	clusterclientv1.ManagedClusterInterface
}

func (m *mockManagedClusterInterface) List(ctx context.Context, opts metav1.ListOptions) (*clusterv1.ManagedClusterList, error) {
	return &clusterList, nil
}

// Mock a managed cluster client that returns an emtpy list
type emptyMockManagedClusterInterface struct {
	clusterclientv1.ManagedClusterInterface
}

func (e *emptyMockManagedClusterInterface) List(ctx context.Context, opts metav1.ListOptions) (*clusterv1.ManagedClusterList, error) {
	return &clusterv1.ManagedClusterList{}, nil
}

// Mock a managed cluster client that returns a not found error
type notFoundMockManagedClusterInterface struct {
	clusterclientv1.ManagedClusterInterface
}

func (e *notFoundMockManagedClusterInterface) List(ctx context.Context, opts metav1.ListOptions) (*clusterv1.ManagedClusterList, error) {
	return nil, notFoundError
}

// Mock a managed cluster client that returns any other error
type errorMockManagedClusterInterface struct {
	clusterclientv1.ManagedClusterInterface
}

func (e *errorMockManagedClusterInterface) List(ctx context.Context, opts metav1.ListOptions) (*clusterv1.ManagedClusterList, error) {
	return nil, testError
}

func TestIncludeNamesFilter(t *testing.T) {
	type args struct {
		filter string
		object metav1.Object
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test IncludeNamesFilter match true",
			args: args{
				filter: "foo",
				object: fooObject,
			},
			want: true,
		},
		{
			name: "Test IncludeNamesFilter match false",
			args: args{
				filter: "foo",
				object: barObject,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(IncludeNamesFilter(tt.args.filter)(tt.args.object), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestExcludeNamesFilter(t *testing.T) {

	type args struct {
		filter string
		object metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test ExcludeNamesFilter match true",
			args: args{
				filter: "foo",
				object: barObject,
			},
			want: true,
		},
		{
			name: "Test ExcludeNamesFilter match false",
			args: args{
				filter: "foo",
				object: fooObject,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(ExcludeNamesFilter(tt.args.filter)(tt.args.object), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}

}

func TestLabelFilter(t *testing.T) {
	type args struct {
		filter map[string]string
		object metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test LabelFilter match true",
			args: args{
				filter: fooLabels,
				object: fooObject,
			},
			want: true,
		},
		{
			name: "Test LabelFilter match false",
			args: args{
				filter: fooLabels,
				object: barObject,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(LabelFilter(tt.args.filter)(tt.args.object), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}

}

func TestClusterClaimsToMap(t *testing.T) {
	type args struct {
		clusterClaims []clusterv1.ManagedClusterClaim
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Test non-empty cluster claims",
			args: args{
				clusterClaims: []clusterv1.ManagedClusterClaim{
					{
						Name:  "foo",
						Value: "bar",
					},
					{
						Name:  "baz",
						Value: "",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
				"baz": "",
			},
		},
		{
			name: "Test empty cluster claims",
			args: args{
				clusterClaims: []clusterv1.ManagedClusterClaim{},
			},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(ClusterClaimsToMap(tt.args.clusterClaims), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestIsValidManagedCluster(t *testing.T) {
	type args struct {
		managedCluster clusterv1.ManagedCluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test OpenShift managed cluster is supported",
			args: args{managedCluster: validOpenShiftCluster},
			want: true,
		},
		{
			name: "Test ROSA managed cluster is supported",
			args: args{managedCluster: validROSACluster},
			want: true,
		},
		{
			name: "Test ARO managed cluster is supported",
			want: true,
			args: args{managedCluster: validAROCluster},
		},
		{
			name: "Test ROKS managed cluster is supported",
			want: true,
			args: args{managedCluster: validROKSCluster},
		},
		{
			name: "Test OpenShiftDedicated managed cluster is supported",
			want: true,
			args: args{managedCluster: validOpenShiftDedicatedCluster},
		},
		{
			name: "Test unsupported managed cluster product",
			want: false,
			args: args{managedCluster: invalidProductCluster},
		},
		{
			name: "Test empty product",
			want: false,
			args: args{managedCluster: emptyProductClaimCluster},
		},
		{
			name: "Test missing product",
			want: false,
			args: args{managedCluster: missingProductClaimCluster},
		},
		{
			name: "Test managed cluster available condition false",
			want: false,
			args: args{managedCluster: availableFalseCluster},
		},
		{
			name: "Test managed cluster available condition unknown",
			want: false,
			args: args{managedCluster: availableUnknownCluster},
		},
		{
			name: "Test managed cluster available condition missing",
			want: false,
			args: args{managedCluster: availableMissingCluster},
		},
		{
			name: "Test managed cluster unsupported version",
			want: false,
			args: args{managedCluster: unsupportedVersionCluster},
		},
		{
			name: "Test malformed version",
			want: false,
			args: args{managedCluster: malformedVersionClaimCluster},
		},
		{
			name: "Test empty version",
			want: false,
			args: args{managedCluster: emptyVersionClaimCluster},
		},
		{
			name: "Test missing version",
			want: false,
			args: args{managedCluster: missingVersionClaimCluster},
		},
		{
			name: "Test empty client config URL",
			want: false,
			args: args{managedCluster: emptyUrlCluster},
		},
		{
			name: "Test missing client config URL",
			want: false,
			args: args{managedCluster: missingUrlCluster},
		},
		{
			name: "Test empty client config CABundle",
			want: false,
			args: args{managedCluster: emptyCABundleCluster},
		},
		{
			name: "Test missing client config CABundle",
			want: false,
			args: args{managedCluster: missingCABundleCluster},
		},
		{
			name: "Test empty client configs",
			want: false,
			args: args{managedCluster: emptyClientConfigCluster},
		},
		{
			name: "Test missing client configs",
			want: false,
			args: args{managedCluster: missingClientConfigCluster},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(IsValidManagedCluster(tt.args.managedCluster), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestGetValidManagedClusters(t *testing.T) {
	type args struct {
		context context.Context
		client  clusterclientv1.ManagedClusterInterface
	}

	type want struct {
		slice  []clusterv1.ManagedCluster
		reason string
		err    error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Test non-empty result",
			args: args{
				context: context.TODO(),
				client:  &mockManagedClusterInterface{},
			},
			want: want{
				slice:  validClusterSlice,
				reason: "",
				err:    nil,
			},
		},
		{
			name: "Test empty result",
			args: args{
				context: context.TODO(),
				client:  &emptyMockManagedClusterInterface{},
			},
			want: want{
				slice:  []clusterv1.ManagedCluster{},
				reason: "",
				err:    nil,
			},
		},
		{
			name: "Test not found",
			args: args{
				context: context.TODO(),
				client:  &notFoundMockManagedClusterInterface{},
			},
			want: want{
				slice:  []clusterv1.ManagedCluster{},
				reason: "",
				err:    nil,
			},
		},
		{
			name: "Test other error",
			args: args{
				context: context.TODO(),
				client:  &errorMockManagedClusterInterface{},
			},
			want: want{
				slice:  []clusterv1.ManagedCluster{},
				reason: "ManagedClusterListError",
				err:    testError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice, reason, err := GetValidManagedClusters(tt.args.context, tt.args.client)
			if diff := deep.Equal(slice, tt.want.slice); diff != nil {
				t.Error(diff)
			}
			if diff := deep.Equal(reason, tt.want.reason); diff != nil {
				t.Error(diff)
			}
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Error(diff)
			}
		})
	}
}
