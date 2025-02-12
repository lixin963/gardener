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

package nodeinit_test

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Init", func() {
	Describe("#Config", func() {
		var (
			worker gardencorev1beta1.Worker
			image  = "gna-repo:gna-tag"

			config              *nodeagentv1alpha1.NodeAgentConfiguration
			oscSecretName       = "osc-secret-name"
			apiServerURL        = "https://localhost"
			caBundle            = []byte("cluster-ca")
			kubernetesVersion   = semver.MustParse("1.2.3")
			oscSyncJitterPeriod = &metav1.Duration{Duration: time.Second}
		)

		BeforeEach(func() {
			worker = gardencorev1beta1.Worker{}
			config = nodeagentcomponent.ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, oscSyncJitterPeriod)
		})

		When("kubelet data volume is not configured", func() {
			It("should return the expected units and files", func() {
				units, files, err := Config(worker, image, config)

				Expect(err).NotTo(HaveOccurred())
				Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
					Name:    nodeagentv1alpha1.InitUnitName,
					Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart),
					Enable:  pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-node-agent/init.sh
[Install]
WantedBy=multi-user.target`),
					Files: []extensionsv1alpha1.File{{
						Path:        "/var/lib/gardener-node-agent/init.sh",
						Permissions: pointer.Int32(0755),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
trap 'ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"' EXIT

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
ctr images pull  "` + image + `" --hosts-dir "/etc/containerd/certs.d"
ctr images mount "` + image + `" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardener-node-agent" "/opt/bin"
chmod +x "/opt/bin/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "/opt/bin/gardener-node-agent" bootstrap --config="/var/lib/gardener-node-agent/config.yaml"
`)),
							},
						},
					}},
				}))
				Expect(files).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/credentials/bootstrap-token",
						Permissions: pointer.Int32(0640),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "<<BOOTSTRAP_TOKEN>>",
							},
							TransmitUnencoded: pointer.Bool(true),
						},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/config.yaml",
						Permissions: pointer.Int32(0600),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
bootstrap: {}
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: ` + kubernetesVersion.String() + `
    secretName: ` + oscSecretName + `
    syncJitterPeriod: ` + oscSyncJitterPeriod.Duration.String() + `
  token:
    secretName: gardener-node-agent
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
					},
				))
			})
		})

		When("kubelet data volume is configured", func() {
			BeforeEach(func() {
				worker.KubeletDataVolumeName = pointer.String("kubelet-data-vol")
				worker.DataVolumes = []gardencorev1beta1.DataVolume{{
					Name:       *worker.KubeletDataVolumeName,
					VolumeSize: "1337Ki",
				}}
			})

			It("should return an error when the data volume cannot be found", func() {
				*worker.KubeletDataVolumeName = "not-found"

				units, files, err := Config(worker, image, config)
				Expect(err).To(MatchError(ContainSubstring("failed finding data volume for kubelet in worker with name")))
				Expect(units).To(BeNil())
				Expect(files).To(BeNil())
			})

			It("should correctly configure the bootstrap configuration", func() {
				_, files, err := Config(worker, image, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ContainElement(extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-node-agent/config.yaml",
					Permissions: pointer.Int32(0600),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
bootstrap:
  kubeletDataVolumeSize: 1369088
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: ` + kubernetesVersion.String() + `
    secretName: ` + oscSecretName + `
    syncJitterPeriod: ` + oscSyncJitterPeriod.Duration.String() + `
  token:
    secretName: gardener-node-agent
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
				}))
			})

			It("should ensure the size of the configuration is not exceeding a certain limit", func() {
				units, files, err := Config(worker, image, config)
				Expect(err).NotTo(HaveOccurred())

				writeFilesToDiskScript, err := operatingsystemconfig.FilesToDiskScript(context.Background(), nil, "", files)
				Expect(err).NotTo(HaveOccurred())
				writeUnitsToDiskScript, err := operatingsystemconfig.UnitsToDiskScript(context.Background(), nil, "", units)
				Expect(err).NotTo(HaveOccurred())

				// best-effort check: ensure the node init configuration is not exceeding 4KB in size
				Expect(utf8.RuneCountInString(writeFilesToDiskScript + writeUnitsToDiskScript)).To(BeNumerically("<", 4096))
			})
		})
	})
})
