// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operatingsystemconfig_test

import (
	"context"
	"io/fs"
	"path"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	fakeregistry "github.com/gardener/gardener/pkg/nodeagent/registry/fake"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("OperatingSystemConfig controller tests", func() {
	var (
		fakeDBus *fakedbus.DBus
		fakeFS   afero.Afero

		oscSecretName     = testRunID
		kubernetesVersion = semver.MustParse("1.2.3")

		node *corev1.Node

		file1, file2, file3, file4, file5                                          extensionsv1alpha1.File
		gnaUnit, unit1, unit2, unit3, unit4, unit5, unit5DropInsOnly, unit6, unit7 extensionsv1alpha1.Unit

		operatingSystemConfig *extensionsv1alpha1.OperatingSystemConfig
		oscRaw                []byte
		oscSecret             *corev1.Secret

		imageMountDirectory string
		cancelFunc          cancelFuncEnsurer
	)

	BeforeEach(func() {
		var err error

		fakeDBus = fakedbus.New()
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}

		imageMountDirectory, err = fakeFS.TempDir("", "fake-node-agent-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(fakeFS.RemoveAll(imageMountDirectory)).To(Succeed()) })

		cancelFunc = cancelFuncEnsurer{}

		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   testRunID,
				Labels: map[string]string{testID: testRunID},
			},
		}

		By("Create Node")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Succeed())
		})

		By("Register controller")
		Expect((&operatingsystemconfig.Reconciler{
			Config: config.OperatingSystemConfigControllerConfig{
				SyncPeriod:        &metav1.Duration{Duration: time.Hour},
				SecretName:        oscSecretName,
				KubernetesVersion: kubernetesVersion,
			},
			DBus:          fakeDBus,
			FS:            fakeFS,
			NodeName:      node.Name,
			Extractor:     fakeregistry.NewExtractor(fakeFS, imageMountDirectory),
			CancelContext: cancelFunc.cancel,
		}).AddToManager(mgr)).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		file1 = extensionsv1alpha1.File{
			Path:        "/example/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file1"}},
			Permissions: pointer.Int32(0777),
		}
		file2 = extensionsv1alpha1.File{
			Path:    "/another/file",
			Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: "ZmlsZTI="}},
		}
		file3 = extensionsv1alpha1.File{
			Path:        "/third/file",
			Content:     extensionsv1alpha1.FileContent{ImageRef: &extensionsv1alpha1.FileContentImageRef{Image: "foo-image", FilePathInImage: "/foo-file"}},
			Permissions: pointer.Int32(0750),
		}
		Expect(fakeFS.WriteFile(path.Join(imageMountDirectory, file3.Content.ImageRef.FilePathInImage), []byte("file3"), 0755)).To(Succeed())
		file4 = extensionsv1alpha1.File{
			Path:        "/unchanged/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file4"}},
			Permissions: pointer.Int32(0750),
		}
		file5 = extensionsv1alpha1.File{
			Path:        "/changed/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file5"}},
			Permissions: pointer.Int32(0750),
		}

		gnaUnit = extensionsv1alpha1.Unit{
			Name:    "gardener-node-agent.service",
			Enable:  pointer.Bool(false),
			Content: pointer.String("#gna"),
		}
		unit1 = extensionsv1alpha1.Unit{
			Name:    "unit1",
			Enable:  pointer.Bool(true),
			Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart),
			Content: pointer.String("#unit1"),
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit1drop",
			}},
		}
		unit2 = extensionsv1alpha1.Unit{
			Name:    "unit2",
			Enable:  pointer.Bool(false),
			Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStop),
			Content: pointer.String("#unit2"),
		}
		unit3 = extensionsv1alpha1.Unit{
			Name: "unit3",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit3drop",
			}},
			Files: []extensionsv1alpha1.File{file4},
		}
		unit4 = extensionsv1alpha1.Unit{
			Name:    "unit4",
			Enable:  pointer.Bool(true),
			Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart),
			Content: pointer.String("#unit4"),
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit4drop",
			}},
		}
		unit5 = extensionsv1alpha1.Unit{
			Name:    "unit5",
			Enable:  pointer.Bool(true),
			Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart),
			Content: pointer.String("#unit5"),
			DropIns: []extensionsv1alpha1.DropIn{
				{
					Name:    "drop1",
					Content: "#unit5drop1",
				},
				{
					Name:    "drop2",
					Content: "#unit5drop2",
				},
			},
		}
		unit5DropInsOnly = extensionsv1alpha1.Unit{
			Name: "unit5",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "extensionsdrop",
				Content: "#unit5extensionsdrop",
			}},
		}
		unit6 = extensionsv1alpha1.Unit{
			Name:    "unit6",
			Enable:  pointer.Bool(true),
			Content: pointer.String("#unit6"),
			Files:   []extensionsv1alpha1.File{file3},
		}
		unit7 = extensionsv1alpha1.Unit{
			Name:    "unit7",
			Enable:  pointer.Bool(true),
			Content: pointer.String("#unit7"),
			Files:   []extensionsv1alpha1.File{file5},
		}

		operatingSystemConfig = &extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Files: []extensionsv1alpha1.File{file1},
				Units: []extensionsv1alpha1.Unit{unit1, unit2, unit5, unit5DropInsOnly, unit6, unit7},
			},
			Status: extensionsv1alpha1.OperatingSystemConfigStatus{
				ExtensionFiles: []extensionsv1alpha1.File{file2},
				ExtensionUnits: []extensionsv1alpha1.Unit{unit3, unit4},
			},
		}

		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		oscSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      oscSecretName,
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{"osc.yaml": oscRaw},
		}
	})

	BeforeEach(func() {
		By("Create Secret containing the operating system config")
		Expect(testClient.Create(ctx, oscSecret)).To(Succeed())

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, oscSecret)).To(Succeed())
		})
	})

	It("should reconcile the configuration when there is no previous OSC", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(And(
			HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)),
			HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()),
		))

		By("Assert that files and units have been created")
		assertFileOnDisk(fakeFS, file1.Path, "file1", 0777)
		assertFileOnDisk(fakeFS, file2.Path, "file2", 0600)
		assertFileOnDisk(fakeFS, file3.Path, "file3", 0750)
		assertFileOnDisk(fakeFS, file4.Path, "file4", 0750)
		assertFileOnDisk(fakeFS, file5.Path, "file5", 0750)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name, "#unit1", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name+".d/"+unit1.DropIns[0].Name, "#unit1drop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name, "#unit2", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit3.Name+".d/"+unit3.DropIns[0].Name, "#unit3drop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name, "#unit4", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name+".d/"+unit4.DropIns[0].Name, "#unit4drop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name, "#unit5", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[0].Name, "#unit5drop1", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[1].Name, "#unit5drop2", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5DropInsOnly.DropIns[0].Name, "#unit5extensionsdrop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit6.Name, "#unit6", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit7.Name, "#unit7", 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit3.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit3.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit7.Name}},
		))

		By("Expect that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when there is a previous OSC", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(And(
			HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)),
			HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()),
		))

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		// manually change permissions of unit and drop-in file (should be restored on next reconciliation)
		Expect(fakeFS.Chmod("/etc/systemd/system/"+unit2.Name, 0777)).To(Succeed())

		By("Update Operating System Config")
		// delete unit1
		// delete file2
		// add drop-in to unit2 and enable+start it
		// disable unit4 and remove all drop-ins
		// remove only first drop-in from unit5
		// move file3 from unit.files to files while keeping it unchanged
		// the content of file5 (belonging to unit7) is changed, so unit7 is restarting
		// file1, unit3, and gardener-node-agent unit are unchanged, so unit3 is not restarting and cancel func is not called
		unit2.Enable = pointer.Bool(true)
		unit2.Command = extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart)
		unit2.DropIns = []extensionsv1alpha1.DropIn{{Name: "dropdropdrop", Content: "#unit2drop"}}
		unit4.Enable = pointer.Bool(false)
		unit4.DropIns = nil
		unit5.DropIns = unit5.DropIns[1:]
		unit6.Files = nil
		unit7.Files[0].Content.Inline.Data = "changeme"

		operatingSystemConfig.Spec.Units = []extensionsv1alpha1.Unit{unit2, unit5, unit6, unit7}
		operatingSystemConfig.Spec.Files = append(operatingSystemConfig.Spec.Files, file3)
		operatingSystemConfig.Status.ExtensionUnits = []extensionsv1alpha1.Unit{unit3, unit4}
		operatingSystemConfig.Status.ExtensionFiles = nil

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(And(
			HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)),
			HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()),
		))

		By("Assert that files and units have been created")
		assertFileOnDisk(fakeFS, file1.Path, "file1", 0777)
		assertNoFileOnDisk(fakeFS, file2.Path)
		assertFileOnDisk(fakeFS, file3.Path, "file3", 0750)
		assertFileOnDisk(fakeFS, file4.Path, "file4", 0750)
		assertFileOnDisk(fakeFS, file5.Path, "changeme", 0750)
		assertNoFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name)
		assertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name+".d")
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name, "#unit2", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name+".d/"+unit2.DropIns[0].Name, "#unit2drop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit3.Name+".d/"+unit3.DropIns[0].Name, "#unit3drop", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name, "#unit4", 0600)
		assertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name+".d")
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name, "#unit5", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[0].Name, "#unit5drop2", 0600)
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit7.Name, "#unit7", 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit7.Name}},
		))

		By("Expect that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should call the cancel function when gardener-node-agent must be restarted itself", func() {
		var lastAppliedOSC []byte
		By("Wait last-applied OSC file to be persisted")
		Eventually(func(g Gomega) error {
			var err error
			lastAppliedOSC, err = fakeFS.ReadFile("/var/lib/gardener-node-agent/last-applied-osc.yaml")
			return err
		}).Should(Succeed())

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		By("Update Operating System Config")
		operatingSystemConfig.Spec.Units = append(operatingSystemConfig.Spec.Units, gnaUnit)

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait last-applied OSC file to be updated")
		Eventually(func(g Gomega) []byte {
			content, err := fakeFS.ReadFile("/var/lib/gardener-node-agent/last-applied-osc.yaml")
			g.Expect(err).NotTo(HaveOccurred())
			return content
		}).ShouldNot(Equal(lastAppliedOSC))

		By("Assert that files and units have been created")
		assertFileOnDisk(fakeFS, "/etc/systemd/system/"+gnaUnit.Name, "#gna", 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{gnaUnit.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
		))

		By("Expect that cancel func has been called")
		Expect(cancelFunc.called).To(BeTrue())
	})
})

func assertFileOnDisk(fakeFS afero.Afero, path, expectedContent string, fileMode uint32) {
	description := "file path " + path

	content, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, string(content)).To(Equal(expectedContent), description)

	fileInfo, err := fakeFS.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(fileMode)), description)
}

func assertNoFileOnDisk(fakeFS afero.Afero, path string) {
	_, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).To(MatchError(afero.ErrFileNotFound), "file path "+path)
}

func assertNoDirectoryOnDisk(fakeFS afero.Afero, path string) {
	exists, err := fakeFS.DirExists(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "directory path "+path)
	ExpectWithOffset(1, exists).To(BeFalse(), "directory path "+path)
}

type cancelFuncEnsurer struct {
	called bool
}

func (c *cancelFuncEnsurer) cancel() {
	c.called = true
}
