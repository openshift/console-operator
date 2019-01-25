// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	console_v1 "github.com/openshift/console-operator/pkg/apis/console/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeConsoleOperatorConfigs implements ConsoleOperatorConfigInterface
type FakeConsoleOperatorConfigs struct {
	Fake *FakeConsoleV1
}

var consoleoperatorconfigsResource = schema.GroupVersionResource{Group: "console.openshift.io", Version: "v1", Resource: "consoleoperatorconfigs"}

var consoleoperatorconfigsKind = schema.GroupVersionKind{Group: "console.openshift.io", Version: "v1", Kind: "ConsoleOperatorConfig"}

// Get takes name of the consoleOperatorConfig, and returns the corresponding consoleOperatorConfig object, and an error if there is any.
func (c *FakeConsoleOperatorConfigs) Get(name string, options v1.GetOptions) (result *console_v1.ConsoleOperatorConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(consoleoperatorconfigsResource, name), &console_v1.ConsoleOperatorConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*console_v1.ConsoleOperatorConfig), err
}

// List takes label and field selectors, and returns the list of ConsoleOperatorConfigs that match those selectors.
func (c *FakeConsoleOperatorConfigs) List(opts v1.ListOptions) (result *console_v1.ConsoleOperatorConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(consoleoperatorconfigsResource, consoleoperatorconfigsKind, opts), &console_v1.ConsoleOperatorConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &console_v1.ConsoleOperatorConfigList{ListMeta: obj.(*console_v1.ConsoleOperatorConfigList).ListMeta}
	for _, item := range obj.(*console_v1.ConsoleOperatorConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested consoleOperatorConfigs.
func (c *FakeConsoleOperatorConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(consoleoperatorconfigsResource, opts))
}

// Create takes the representation of a consoleOperatorConfig and creates it.  Returns the server's representation of the consoleOperatorConfig, and an error, if there is any.
func (c *FakeConsoleOperatorConfigs) Create(consoleOperatorConfig *console_v1.ConsoleOperatorConfig) (result *console_v1.ConsoleOperatorConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(consoleoperatorconfigsResource, consoleOperatorConfig), &console_v1.ConsoleOperatorConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*console_v1.ConsoleOperatorConfig), err
}

// Update takes the representation of a consoleOperatorConfig and updates it. Returns the server's representation of the consoleOperatorConfig, and an error, if there is any.
func (c *FakeConsoleOperatorConfigs) Update(consoleOperatorConfig *console_v1.ConsoleOperatorConfig) (result *console_v1.ConsoleOperatorConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(consoleoperatorconfigsResource, consoleOperatorConfig), &console_v1.ConsoleOperatorConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*console_v1.ConsoleOperatorConfig), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeConsoleOperatorConfigs) UpdateStatus(consoleOperatorConfig *console_v1.ConsoleOperatorConfig) (*console_v1.ConsoleOperatorConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(consoleoperatorconfigsResource, "status", consoleOperatorConfig), &console_v1.ConsoleOperatorConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*console_v1.ConsoleOperatorConfig), err
}

// Delete takes name of the consoleOperatorConfig and deletes it. Returns an error if one occurs.
func (c *FakeConsoleOperatorConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(consoleoperatorconfigsResource, name), &console_v1.ConsoleOperatorConfig{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeConsoleOperatorConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(consoleoperatorconfigsResource, listOptions)

	_, err := c.Fake.Invokes(action, &console_v1.ConsoleOperatorConfigList{})
	return err
}

// Patch applies the patch and returns the patched consoleOperatorConfig.
func (c *FakeConsoleOperatorConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *console_v1.ConsoleOperatorConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(consoleoperatorconfigsResource, name, data, subresources...), &console_v1.ConsoleOperatorConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*console_v1.ConsoleOperatorConfig), err
}