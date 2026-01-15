// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *LoginBuilder) BuildLoginSshConfig(loginset *slinkyv1beta1.LoginSet) (*corev1.ConfigMap, error) {
	spec := loginset.Spec
	opts := common.ConfigMapOpts{
		Key: loginset.SshConfigKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: loginset.Annotations,
			Labels:      structutils.MergeMaps(loginset.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build()),
		},
		Data: map[string]string{
			authorizedKeysFile: buildAuthorizedKeys(spec.RootSshAuthorizedKeys),
			SshdConfigFile:     buildSshdConfig(spec.ExtraSshdConfig),
		},
	}

	return b.CommonBuilder.BuildConfigMap(opts, loginset)
}

func buildAuthorizedKeys(authorizedKeys string) string {
	conf := config.NewBuilder()

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### SLINKY ###"))
	conf.AddProperty(config.NewPropertyRaw(authorizedKeys))

	return conf.Build()
}

func buildSshdConfig(extraConf string) string {
	conf := config.NewBuilder().WithSeperator(" ")

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### GENERAL ###"))
	conf.AddProperty(config.NewProperty("Include", "/etc/ssh/sshd_config.d/*.conf"))
	conf.AddProperty(config.NewProperty("UsePAM", "yes"))
	conf.AddProperty(config.NewProperty("X11Forwarding", "yes"))
	conf.AddProperty(config.NewProperty("Subsystem", "sftp internal-sftp"))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
	conf.AddProperty(config.NewPropertyRaw(extraConf))

	return conf.Build()
}
