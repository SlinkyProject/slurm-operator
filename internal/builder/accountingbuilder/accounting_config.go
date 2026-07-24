// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	"context"
	"fmt"
	"strings"

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

	secret, err := b.CommonBuilder.BuildSecret(opts, accounting)
	if err != nil {
		return secret, fmt.Errorf("failed to build secret: %w", err)
	}

	return secret, nil
}

// https://slurm.schedmd.com/slurmdbd.conf.html
func buildSlurmdbdConf(accounting *slinkyv1beta1.Accounting, storagePass string) string {
	mergeConfig := map[string][]string{
		"AuthInfo": {
			common.AuthInfo,
		},
		"AuthAltParameters": func() []string {
			params := []string{common.JwtAuthAltParameters}
			if accounting.AuthJwksRef() != nil {
				params = append(params, common.JwksAuthAltParameters)
			}
			return params
		}(),
	}

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
	conf.AddProperty(config.NewProperty("AuthAltParameters", strings.Join(mergeConfig["AuthAltParameters"], ",")))
	conf.AddProperty(config.NewProperty("AuthInfo", strings.Join(mergeConfig["AuthInfo"], ",")))

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

	extraConf := accounting.Spec.ExtraConf
	if extraConf != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
		conf.AddProperty(config.NewPropertyRaw(extraConf))
	}

	if snippet := common.BuildMergedConfig(extraConf, mergeConfig); snippet != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### MERGED CONFIG ###"))
		conf.AddProperty(config.NewPropertyRaw(snippet))
	}

	return conf.Build()
}
