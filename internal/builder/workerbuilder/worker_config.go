// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	loginbuilder "github.com/SlinkyProject/slurm-operator/internal/builder/loginbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *WorkerBuilder) BuildWorkerSshConfig(nodeset *slinkyv1beta1.NodeSet) (*corev1.ConfigMap, error) {
	opts := common.ConfigMapOpts{
		Key: nodeset.SshConfigKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: nodeset.Annotations,
			Labels:      structutils.MergeMaps(nodeset.Labels, labels.NewBuilder().WithWorkerLabels(nodeset).Build()),
		},
		Data: map[string]string{
			loginbuilder.SshdConfigFile: buildWorkerSshdConfig(nodeset.Spec.Ssh.ExtraSshdConfig),
		},
	}

	return b.CommonBuilder.BuildConfigMap(opts, nodeset)
}

// Ref: https://slurm.schedmd.com/pam_slurm_adopt.html#ssh_config
func buildWorkerSshdConfig(extraConf string) string {
	conf := config.NewBuilder().WithSeperator(" ")

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### GENERAL ###"))
	conf.AddProperty(config.NewProperty("Include", "/etc/ssh/sshd_config.d/*.conf"))
	conf.AddProperty(config.NewProperty("UsePAM", "yes"))
	conf.AddProperty(config.NewProperty("Subsystem", "sftp internal-sftp"))
	conf.AddProperty(config.NewProperty("AuthenticationMethods", "publickey password"))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
	conf.AddProperty(config.NewPropertyRaw(extraConf))

	return conf.Build()
}
