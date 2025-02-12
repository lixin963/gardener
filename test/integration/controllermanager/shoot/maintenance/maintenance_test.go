// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package maintenance_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Shoot Maintenance controller tests", func() {
	var (
		cloudProfile *gardencorev1beta1.CloudProfile
		shoot        *gardencorev1beta1.Shoot
		shoot126     *gardencorev1beta1.Shoot

		// Test Machine Image
		machineImageName             = "foo-image"
		highestVersionNextMajorARM   = "2.1.1"
		highestVersionNextMajorAMD64 = "2.1.3"

		highestVersionForCurrentMajorARM   = "0.4.1"
		highestVersionForCurrentMajorAMD64 = "0.3.0"

		highestPatchNextMinorARM   = "0.2.4"
		highestPatchNextMinorAMD64 = "0.2.3"

		highestSupportedARMVersion = "2.0.0"

		highestPatchSameMinorARM   = "0.0.4"
		highestPatchSameMinorAMD64 = "0.0.3"
		overallLatestVersionARM    = "3.0.0"
		overallLatestVersionAMD64  = "3.0.1"

		testMachineImageVersion = "0.0.1-beta"
		testMachineImage        = gardencorev1beta1.ShootMachineImage{
			Name:    machineImageName,
			Version: &testMachineImageVersion,
		}

		// other
		deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		supportedClassification  = gardencorev1beta1.ClassificationSupported
		expirationDateInThePast  = metav1.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

		// Test Kubernetes versions
		testKubernetesVersionLowPatchLowMinor             = gardencorev1beta1.ExpirableVersion{Version: "0.0.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchLowMinor         = gardencorev1beta1.ExpirableVersion{Version: "0.0.5", Classification: &deprecatedClassification}
		testKubernetesVersionLowPatchConsecutiveMinor     = gardencorev1beta1.ExpirableVersion{Version: "0.1.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.5", Classification: &deprecatedClassification}
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now().Round(time.Second))

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.25.1",
						},
						{
							Version: "1.26.0",
						},
						{
							Version: "1.27.0",
						},
						testKubernetesVersionLowPatchLowMinor,
						testKubernetesVersionHighestPatchLowMinor,
						testKubernetesVersionLowPatchConsecutiveMinor,
						testKubernetesVersionHighestPatchConsecutiveMinor,
					},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: machineImageName,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// ARM overall latest version: 3.0.0
									Version:        overallLatestVersionARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64 overall latest version: 3.0.1
									Version:        overallLatestVersionAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest ARM {patch, minor} of next major: 2.1.1
									Version:        highestVersionNextMajorARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest AMD64 {patch, minor} of next major: 2.1.3
									Version:        highestVersionNextMajorAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// patch update should never be forcefully updated to the next major version
									Version:        highestSupportedARMVersion,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest patch for minor: 0.4.1
									Version:        highestVersionForCurrentMajorARM,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest patch for minor: 0.3.0
									Version:        highestVersionForCurrentMajorAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// ARM: highest patch version for next higher minor version
									// should be force-updated to this version: 0.2.4
									Version:        highestPatchNextMinorARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64: highest patch version for next higher minor version
									// should be force-updated to this version: 0.2.3
									Version:        highestPatchNextMinorAMD64,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "0.2.0",
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// ARM highest patch version for Shoot's minor: 0.0.4
									Version:        highestPatchSameMinorARM,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64 highest patch version for Shoot's minor: 0.0.3
									Version:        highestPatchSameMinorAMD64,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "0.0.2",
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        testMachineImageVersion,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name: "large",
					},
				},
				Regions: []gardencorev1beta1.Region{
					{
						Name: "foo-region",
					},
				},
				Type: "foo-type",
			},
		}

		By("Create CloudProfile")
		Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
		log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

		DeferCleanup(func() {
			By("Delete CloudProfile")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("my-provider-account"),
				CloudProfileName:  cloudProfile.Name,
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker1",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Image: &testMachineImage,
								Type:  "large",
							},
						},
						{
							Name:    "cpu-worker2",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Image:        &testMachineImage,
								Type:         "large",
								Architecture: pointer.String("arm64"),
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo-networking"),
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
						KubernetesVersion:   false,
						MachineImageVersion: pointer.Bool(false),
					},
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: timewindow.NewMaintenanceTime(time.Now().Add(2*time.Hour).Hour(), 0, 0).Formatted(),
						End:   timewindow.NewMaintenanceTime(time.Now().Add(4*time.Hour).Hour(), 0, 0).Formatted(),
					},
				},
			},
		}

		shoot126 = shoot.DeepCopy()
		// set dummy kubernetes version to shoot
		shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchLowMinor.Version

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})
	})

	It("should add task annotations", func() {
		By("Trigger maintenance")
		Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

		waitForShootToBeMaintained(shoot)

		By("Ensure task annotations are present")
		Expect(shoot.Annotations).To(HaveKey("shoot.gardener.cloud/tasks"))
		Expect(strings.Split(shoot.Annotations["shoot.gardener.cloud/tasks"], ",")).To(And(
			ContainElement("deployInfrastructure"),
			ContainElement("deployDNSRecordInternal"),
			ContainElement("deployDNSRecordExternal"),
			ContainElement("deployDNSRecordIngress"),
		))
	})

	Context("operation annotations", func() {
		var oldGeneration int64

		BeforeEach(func() {
			oldGeneration = shoot.Generation
		})

		Context("failed last operation state", func() {
			BeforeEach(func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should not set the retry operation annotation due to missing 'needs-retry-operation' annotation", func() {
				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should set the retry operation annotation due to 'needs-retry-operation' annotation (implicitly increasing the generation)", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.shoot.gardener.cloud/needs-retry-operation", "true")
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
				Expect(shoot.Annotations["maintenance.shoot.gardener.cloud/needs-retry-operation"]).To(BeEmpty())
			})
		})

		Context("non-failed last operation states", func() {
			BeforeEach(func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should set the reconcile operation annotation (implicitly increasing the generation)", func() {
				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should set the maintenance operation annotation if it's valid", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-kubeconfig-credentials")
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(Equal("rotate-kubeconfig-credentials"))
				Expect(shoot.Annotations["maintenance.gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should not set the maintenance operation annotation if it's invalid and use the reconcile operation instead", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "foo-bar-does-not-exist")
				err := testClient.Patch(ctx, shoot, patch)
				Expect(apierrors.IsInvalid(err)).To(BeTrue())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
				Expect(shoot.Annotations["maintenance.gardener.cloud/operation"]).To(BeEmpty())
			})
		})
	})

	Describe("Machine image maintenance tests", func() {

		It("Do not update Shoot machine image in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate does not apply", func() {
			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: testMachineImage.Name, Version: testMachineImage.Version}))
				g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: testMachineImage.Name, Version: testMachineImage.Version}))
			}).Should(Succeed())
		})

		Describe("AutoUpdateStrategy: major (default)", func() {
			BeforeEach(func() {
				By("Set updateStrategy: major for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMajor
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest overall version (update strategy: major)", func() {
				By("Set autoupdate=true and Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				cloneShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(true)
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestSupportedARMVersion}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Automatic update of the machine image version is configured (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest overall version because the machine image is expired (update strategy: major)", func() {
				By("Set Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					// updates for strategy major prefer upgrading to a supported version. Here, the latest ARM supported version is version "2.0.0" and the ARM latest overall version
					// is a deprecated version "3.0.0"
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestSupportedARMVersion}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Machine image version expired - force update required (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("updating one worker pool fails, while the other one succeeds (update strategy: major)", func() {
				By("Set Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &overallLatestVersionARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				cpPatch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMajor
				Expect(testClient.Patch(ctx, cloudProfile, cpPatch)).To(Succeed())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &overallLatestVersionARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("(1/2) maintenance operations successful"))
					g.Expect(shoot.Status.LastMaintenance.FailureReason).ToNot(BeNil())
					g.Expect(*shoot.Status.LastMaintenance.FailureReason).To(ContainSubstring("Worker pool \"cpu-worker2\": failed to update machine image"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})

			It("force update to latest patch version in minor, as the current version is not the latest for the current minor (update strategy: major)", func() {
				By("Expire Shoot's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testMachineImage.Name, *testMachineImage.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchSameMinorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestPatchSameMinorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Machine image version expired - force update required (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

		})

		Describe("AutoUpdateStrategy: patch", func() {
			BeforeEach(func() {
				By("Set updateStrategy: major for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyPatch := gardencorev1beta1.UpdateStrategyPatch
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyPatch
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest patch version in minor (update strategy: patch)", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(true)
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: pointer.String(highestPatchSameMinorAMD64)}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: pointer.String(highestPatchSameMinorARM)}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest patch version in minor - not on latest patch version yet (update strategy: patch)", func() {
				By("Expire Shoot's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testMachineImage.Name, *testMachineImage.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchSameMinorAMD64}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in next minor because the machine image is expired (update strategy: patch)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the minor")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchNextMinorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestPatchNextMinorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", highestPatchSameMinorAMD64, highestPatchNextMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", highestPatchSameMinorARM, highestPatchNextMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("fail to force update because already on latest patch in minor (update strategy: patch)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForCurrentMajorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestVersionForCurrentMajorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionForCurrentMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionForCurrentMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})
		})

		Describe("AutoUpdateStrategy: minor", func() {
			BeforeEach(func() {
				By("Set updateStrategy: minor for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMinor
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest patch version in major (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the minor (so does not upgrade to latest in minor first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(true)
				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchNextMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchNextMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: pointer.String(highestVersionForCurrentMajorAMD64)}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: pointer.String(highestVersionForCurrentMajorARM)}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: minor)", highestPatchNextMinorAMD64, highestVersionForCurrentMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: minor)", highestPatchNextMinorARM, highestVersionForCurrentMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in current major - not on latest in current major yet (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major (so does not upgrade to latest in current major first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionForCurrentMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionForCurrentMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestPatchSameMinorAMD64, highestVersionForCurrentMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestPatchSameMinorARM, highestVersionForCurrentMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in the next major (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major (so does not upgrade to latest in current major first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForCurrentMajorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestVersionForCurrentMajorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionNextMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionNextMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestVersionForCurrentMajorAMD64, highestVersionNextMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestVersionForCurrentMajorARM, highestVersionNextMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("fail to force update because already on overall latest version (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &overallLatestVersionAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &overallLatestVersionARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &overallLatestVersionARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})
		})
	})

	Describe("Kubernetes version maintenance tests", func() {
		BeforeEach(func() {
			shoot126.Spec.Kubernetes.Version = "1.26.0"
			shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)

			By("Create k8s v1.26 Shoot")
			Expect(testClient.Create(ctx, shoot126)).To(Succeed())
			log.Info("Created shoot with k8s v1.26 for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot with k8s v1.26")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot126))).To(Succeed())
			})
		})

		Context("Shoot with worker", func() {
			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.27) and set EnableStaticTokenKubeconfig value to false", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot126, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot126), shoot126)).To(Succeed())
					g.Expect(shoot126.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot126.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.26.0\" to \"1.27.0\". Reason: Kubernetes version expired - force update required, EnableStaticTokenKubeconfig is set to false. Reason: The static token kubeconfig can no longer be enabled for Shoot clusters using Kubernetes version 1.27 and higher"))
					g.Expect(shoot126.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot126.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(Equal(pointer.Bool(false)))
					return shoot126.Spec.Kubernetes.Version
				}).Should(Equal("1.27.0"))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})
		})

		Describe("Worker Pool Kubernetes version maintenance tests", func() {
			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("All maintenance operations successful. Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				// expire the Shoot's Kubernetes version because autoupdate is set to false
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated, but control plane version stays: force update minor of worker pool version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchConsecutiveMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's worker pool kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				// expire the Shoot's Kubernetes version because autoupdate is set to false
				patch := client.MergeFrom(shoot.DeepCopy())
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.27) and set EnableStaticTokenKubeconfig value to false", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot126, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot126), shoot126)).To(Succeed())
					g.Expect(shoot126.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot126.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.26.0\" to \"1.27.0\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot126.Status.LastMaintenance.Description).To(ContainSubstring("EnableStaticTokenKubeconfig is set to false. Reason: The static token kubeconfig can no longer be enabled for Shoot clusters using Kubernetes version 1.27 and higher"))
					g.Expect(shoot126.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot126.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(Equal(pointer.Bool(false)))
					return shoot126.Spec.Kubernetes.Version
				}).Should(Equal("1.27.0"))
			})
		})
	})
})
