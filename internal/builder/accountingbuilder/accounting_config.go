// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	common "github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *AccountingBuilder) BuildAccountingConfig(accounting *slinkyv1beta1.Accounting) (*corev1.Secret, error) {
	storagePass, err := b.refResolver.GetSecretKeyRef(context.TODO(), accounting.AuthStorageRef(), accounting.Namespace)
	if err != nil {
		return nil, err
	}

	opts := common.SecretOpts{
		Key: accounting.ConfigKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: accounting.Annotations,
			Labels:      structutils.MergeMaps(accounting.Labels, labels.NewBuilder().WithAccountingLabels(accounting).Build()),
		},
		StringData: map[string]string{
			common.SlurmdbdConfFile: buildSlurmdbdConf(accounting, string(storagePass)),
		},
	}

	return b.CommonBuilder.BuildSecret(opts, accounting)
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
	conf.AddProperty(config.NewProperty("DbdPort", common.SlurmdbdPort))
	conf.AddProperty(config.NewProperty("SlurmUser", common.SlurmUser))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### PLUGINS & PARAMETERS ###"))
	conf.AddProperty(config.NewProperty("AuthType", common.AuthType))
	conf.AddProperty(config.NewProperty("AuthAltTypes", common.AuthAltTypes))
	conf.AddProperty(config.NewProperty("AuthAltParameters", common.AuthAltParameters))
	conf.AddProperty(config.NewProperty("AuthInfo", common.AuthInfo))

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
	conf.AddProperty(config.NewProperty("LogFile", common.DevNull))
	conf.AddProperty(config.NewProperty("LogTimeFormat", common.LogTimeFormat))

	extraConf := accounting.Spec.ExtraConf
	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
	conf.AddProperty(config.NewPropertyRaw(extraConf))

	return conf.Build()
}
