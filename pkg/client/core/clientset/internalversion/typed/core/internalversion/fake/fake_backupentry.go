/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	core "github.com/gardener/gardener/pkg/apis/core"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBackupEntries implements BackupEntryInterface
type FakeBackupEntries struct {
	Fake *FakeCore
	ns   string
}

var backupentriesResource = core.SchemeGroupVersion.WithResource("backupentries")

var backupentriesKind = core.SchemeGroupVersion.WithKind("BackupEntry")

// Get takes name of the backupEntry, and returns the corresponding backupEntry object, and an error if there is any.
func (c *FakeBackupEntries) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.BackupEntry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(backupentriesResource, c.ns, name), &core.BackupEntry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupEntry), err
}

// List takes label and field selectors, and returns the list of BackupEntries that match those selectors.
func (c *FakeBackupEntries) List(ctx context.Context, opts v1.ListOptions) (result *core.BackupEntryList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(backupentriesResource, backupentriesKind, c.ns, opts), &core.BackupEntryList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &core.BackupEntryList{ListMeta: obj.(*core.BackupEntryList).ListMeta}
	for _, item := range obj.(*core.BackupEntryList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested backupEntries.
func (c *FakeBackupEntries) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(backupentriesResource, c.ns, opts))

}

// Create takes the representation of a backupEntry and creates it.  Returns the server's representation of the backupEntry, and an error, if there is any.
func (c *FakeBackupEntries) Create(ctx context.Context, backupEntry *core.BackupEntry, opts v1.CreateOptions) (result *core.BackupEntry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(backupentriesResource, c.ns, backupEntry), &core.BackupEntry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupEntry), err
}

// Update takes the representation of a backupEntry and updates it. Returns the server's representation of the backupEntry, and an error, if there is any.
func (c *FakeBackupEntries) Update(ctx context.Context, backupEntry *core.BackupEntry, opts v1.UpdateOptions) (result *core.BackupEntry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(backupentriesResource, c.ns, backupEntry), &core.BackupEntry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupEntry), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeBackupEntries) UpdateStatus(ctx context.Context, backupEntry *core.BackupEntry, opts v1.UpdateOptions) (*core.BackupEntry, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(backupentriesResource, "status", c.ns, backupEntry), &core.BackupEntry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupEntry), err
}

// Delete takes name of the backupEntry and deletes it. Returns an error if one occurs.
func (c *FakeBackupEntries) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(backupentriesResource, c.ns, name, opts), &core.BackupEntry{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBackupEntries) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(backupentriesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &core.BackupEntryList{})
	return err
}

// Patch applies the patch and returns the patched backupEntry.
func (c *FakeBackupEntries) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.BackupEntry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(backupentriesResource, c.ns, name, pt, data, subresources...), &core.BackupEntry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupEntry), err
}
