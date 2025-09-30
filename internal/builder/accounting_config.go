// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *Builder) BuildAccountingConfig(accounting *slinkyv1beta1.Accounting) (*corev1.Secret, error) {
	storagePass, err := b.refResolver.GetSecretKeyRef(context.TODO(), accounting.AuthStorageRef(), accounting.Namespace)
	if err != nil {
		return nil, err
	}

	opts := SecretOpts{
		Key:      accounting.ConfigKey(),
		Metadata: accounting.Spec.Template.PodMetadata,
		StringData: map[string]string{
			slurmdbdConfFile: buildSlurmdbdConf(accounting, string(storagePass)),
		},
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithAccountingLabels(accounting).Build())

	return b.BuildSecret(opts, accounting)
}

// https://slurm.schedmd.com/slurmdbd.conf.html
func buildSlurmdbdConf(accounting *slinkyv1beta1.Accounting, storagePass string) string {
	dbdHost := accounting.PrimaryName()
	storageHost := accounting.Spec.StorageConfig.Host
	storagePort := accounting.Spec.StorageConfig.Port
	storageLoc := accounting.Spec.StorageConfig.Database
	storageUser := accounting.Spec.StorageConfig.Username

	conf := config.NewBuilder()

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### GENERAL ###"))
	conf.AddProperty(config.NewProperty("DbdHost", dbdHost))
	conf.AddProperty(config.NewProperty("DbdPort", SlurmdbdPort))
	conf.AddProperty(config.NewProperty("SlurmUser", slurmUser))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### PLUGINS & PARAMETERS ###"))
	conf.AddProperty(config.NewProperty("AuthType", authType))
	conf.AddProperty(config.NewProperty("AuthAltTypes", authAltTypes))
	conf.AddProperty(config.NewProperty("AuthAltParameters", authAltParameters))
	conf.AddProperty(config.NewProperty("AuthInfo", authInfo))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### STORAGE ###"))
	conf.AddProperty(config.NewProperty("StorageType", "accounting_storage/mysql"))
	conf.AddProperty(config.NewProperty("StorageHost", storageHost))
	conf.AddProperty(config.NewProperty("StoragePort", storagePort))
	conf.AddProperty(config.NewProperty("StorageUser", storageUser))
	conf.AddProperty(config.NewProperty("StorageLoc", storageLoc))
	conf.AddProperty(config.NewProperty("StoragePass", storagePass))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### LOGGING ###"))
	conf.AddProperty(config.NewProperty("LogFile", devNull))
	conf.AddProperty(config.NewProperty("LogTimeFormat", logTimeFormat))

	extraConf := accounting.Spec.ExtraConf
	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
	conf.AddProperty(config.NewPropertyRaw(extraConf))

	return conf.Build()
}
