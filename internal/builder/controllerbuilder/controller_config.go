// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

const (
	SlurmConfFile  = "slurm.conf"
	CgroupConfFile = "cgroup.conf"
	GresConfFile   = "gres.conf"
)

func (b *ControllerBuilder) BuildControllerConfig(controller *slinkyv1beta1.Controller) (*corev1.ConfigMap, error) {
	ctx := context.TODO()

	var accounting *slinkyv1beta1.Accounting
	if controller.Spec.AccountingRef != nil {
		var err error
		accounting, err = b.refResolver.GetAccounting(ctx, *controller.Spec.AccountingRef)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	nodesetList, err := b.refResolver.GetNodeSetsForController(ctx, controller)
	if err != nil {
		return nil, err
	}

	configFilesList := &corev1.ConfigMapList{
		Items: make([]corev1.ConfigMap, 0, len(controller.Spec.ConfigFileRefs)),
	}
	for _, ref := range controller.Spec.ConfigFileRefs {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      ref.Name,
		}
		if err := b.client.Get(ctx, key, cm); err != nil {
			return nil, err
		}
		configFilesList.Items = append(configFilesList.Items, *cm)
	}
	cgroupEnabled := true
	hasCgroupConfFile := false
	hasGresConfFile := false
	for _, configMap := range configFilesList.Items {
		if contents, ok := configMap.Data[CgroupConfFile]; ok {
			hasCgroupConfFile = true
			cgroupEnabled = isCgroupEnabled(contents)
		}
		if _, ok := configMap.Data[GresConfFile]; ok {
			hasGresConfFile = true
		}
	}

	prologScripts := []string{}
	for _, ref := range controller.Spec.PrologScriptRefs {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      ref.Name,
		}
		if err := b.client.Get(ctx, key, cm); err != nil {
			return nil, err
		}
		filenames := structutils.Keys(cm.Data)
		sort.Strings(filenames)
		prologScripts = append(prologScripts, filenames...)
	}

	epilogScripts := []string{}
	for _, ref := range controller.Spec.EpilogScriptRefs {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      ref.Name,
		}
		if err := b.client.Get(ctx, key, cm); err != nil {
			return nil, err
		}
		filenames := structutils.Keys(cm.Data)
		sort.Strings(filenames)
		epilogScripts = append(epilogScripts, filenames...)
	}

	prologSlurmctldScripts := []string{}
	for _, ref := range controller.Spec.PrologSlurmctldScriptRefs {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      ref.Name,
		}
		if err := b.client.Get(ctx, key, cm); err != nil {
			return nil, err
		}
		filenames := structutils.Keys(cm.Data)
		sort.Strings(filenames)
		prologSlurmctldScripts = append(prologSlurmctldScripts, filenames...)
	}

	epilogSlurmctldScripts := []string{}
	for _, ref := range controller.Spec.EpilogSlurmctldScriptRefs {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      ref.Name,
		}
		if err := b.client.Get(ctx, key, cm); err != nil {
			return nil, err
		}
		filenames := structutils.Keys(cm.Data)
		sort.Strings(filenames)
		epilogSlurmctldScripts = append(epilogSlurmctldScripts, filenames...)
	}

	opts := common.ConfigMapOpts{
		Key: controller.ConfigKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: controller.Annotations,
			Labels:      structutils.MergeMaps(controller.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		Data: map[string]string{
			SlurmConfFile: buildSlurmConf(
				controller, accounting, nodesetList,
				prologScripts, epilogScripts,
				prologSlurmctldScripts, epilogSlurmctldScripts,
				cgroupEnabled),
		},
	}
	if !hasCgroupConfFile {
		opts.Data[CgroupConfFile] = buildCgroupConf()
	}
	if !hasGresConfFile {
		opts.Data[GresConfFile] = buildGresConf()
	}

	return b.CommonBuilder.BuildConfigMap(opts, controller)
}

// https://slurm.schedmd.com/slurm.conf.html
func buildSlurmConf(
	controller *slinkyv1beta1.Controller,
	accounting *slinkyv1beta1.Accounting,
	nodesetList *slinkyv1beta1.NodeSetList,
	prologScripts, epilogScripts []string,
	prologSlurmctldScripts, epilogSlurmctldScripts []string,
	cgroupEnabled bool,
) string {
	mergeConfig := map[string][]string{
		"SlurmctldParameters": func() []string {
			params := []string{"enable_configless"}
			if cgroupEnabled {
				params = append(params, "enable_stepmgr")
			}
			return params
		}(),
		"AuthInfo": {
			common.AuthInfo,
		},
		"AuthAltParameters": func() []string {
			params := []string{common.JwtAuthAltParameters}
			if controller.AuthJwksRef() != nil {
				params = append(params, common.JwksAuthAltParameters)
			}
			return params
		}(),
	}

	controllerHost := fmt.Sprintf("%s(%s)", controller.PrimaryName(), controller.ServiceFQDNShort())

	conf := config.NewBuilder()

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### GENERAL ###"))
	conf.AddProperty(config.NewProperty("ClusterName", controller.ClusterName()))
	conf.AddProperty(config.NewProperty("SlurmUser", common.SlurmUser))
	conf.AddProperty(config.NewProperty("SlurmctldHost", controllerHost))
	conf.AddProperty(config.NewProperty("SlurmctldPort", common.SlurmctldPort))
	conf.AddProperty(config.NewProperty("StateSaveLocation", clusterSpoolDir(controller.ClusterName())))
	conf.AddProperty(config.NewProperty("SlurmdUser", common.SlurmdUser))
	conf.AddProperty(config.NewProperty("SlurmdPort", common.SlurmdPort))
	conf.AddProperty(config.NewProperty("SlurmdSpoolDir", common.SlurmdSpoolDir))
	conf.AddProperty(config.NewProperty("ReturnToService", 2))
	conf.AddProperty(config.NewProperty("MaxNodeCount", 1024))
	conf.AddProperty(config.NewProperty("GresTypes", "gpu"))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### LOGGING ###"))
	conf.AddProperty(config.NewProperty("SlurmctldLogFile", common.SlurmctldLogFilePath))
	conf.AddProperty(config.NewProperty("SlurmSchedLogFile", common.SlurmctldLogFilePath))
	conf.AddProperty(config.NewProperty("SlurmdLogFile", common.SlurmdLogFilePath))
	conf.AddProperty(config.NewProperty("LogTimeFormat", common.LogTimeFormat))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### PLUGINS & PARAMETERS ###"))
	conf.AddProperty(config.NewProperty("AuthType", common.AuthType))
	conf.AddProperty(config.NewProperty("CredType", common.CredType))
	conf.AddProperty(config.NewProperty("AuthAltTypes", common.AuthAltTypes))
	conf.AddProperty(config.NewProperty("AuthAltParameters", strings.Join(mergeConfig["AuthAltParameters"], ",")))
	conf.AddProperty(config.NewProperty("AuthInfo", strings.Join(mergeConfig["AuthInfo"], ",")))
	conf.AddProperty(config.NewProperty("SlurmctldParameters", strings.Join(mergeConfig["SlurmctldParameters"], ",")))
	if cgroupEnabled {
		conf.AddProperty(config.NewProperty("ProctrackType", "proctrack/cgroup"))
		conf.AddProperty(config.NewProperty("PrologFlags", "Contain"))
		conf.AddProperty(config.NewProperty("TaskPlugin", "task/affinity,task/cgroup"))
	} else {
		conf.AddProperty(config.NewProperty("ProctrackType", "proctrack/linuxproc"))
		conf.AddProperty(config.NewProperty("TaskPlugin", "task/affinity"))
	}

	metricsEnabled := controller.Spec.Metrics.Enabled
	if metricsEnabled {
		conf.AddProperty(config.NewProperty("MetricsType", "metrics/openmetrics"))
	}

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### ACCOUNTING ###"))
	if accounting != nil {
		conf.AddProperty(config.NewProperty("AccountingStorageType", "accounting_storage/slurmdbd"))
		conf.AddProperty(config.NewProperty("AccountingStorageHost", accounting.ServiceKey().Name))
		conf.AddProperty(config.NewProperty("AccountingStoragePort", common.SlurmdbdPort))
		conf.AddProperty(config.NewProperty("AccountingStorageTRES", "gres/gpu"))
		if cgroupEnabled {
			conf.AddProperty(config.NewProperty("JobAcctGatherType", "jobacct_gather/cgroup"))
		} else {
			conf.AddProperty(config.NewProperty("JobAcctGatherType", "jobacct_gather/linux"))
		}
	} else {
		conf.AddProperty(config.NewProperty("AccountingStorageType", "accounting_storage/none"))
		conf.AddProperty(config.NewProperty("JobAcctGatherType", "jobacct_gather/none"))
	}

	if snippet := buildPrologEpilogSlurmctldConf(prologSlurmctldScripts, epilogSlurmctldScripts); snippet != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### SLURMCTLD PROLOG & EPILOG ###"))
		conf.AddProperty(config.NewPropertyRaw(snippet))
	}

	if snippet := buildPrologEpilogConf(prologScripts, epilogScripts); snippet != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### PROLOG & EPILOG ###"))
		conf.AddProperty(config.NewPropertyRaw(snippet))
	}

	if snippet := buildNodeSetConf(nodesetList); snippet != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### NODESET & PARTITION ###"))
		conf.AddProperty(config.NewPropertyRaw(snippet))
	}

	extraConf := controller.Spec.ExtraConf
	filteredExtraConf := filterExtraConfPartitions(extraConf, nodeSetNames(nodesetList))
	if filteredExtraConf != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### EXTRA CONFIG ###"))
		conf.AddProperty(config.NewPropertyRaw(filteredExtraConf))
	}

	if snippet := common.BuildMergedConfig(extraConf, mergeConfig); snippet != "" {
		conf.AddProperty(config.NewPropertyRaw("#"))
		conf.AddProperty(config.NewPropertyRaw("### MERGED CONFIG ###"))
		conf.AddProperty(config.NewPropertyRaw(snippet))
	}

	return conf.Build()
}

// buildPrologEpilogConf() returns a slurm.conf snippet containing PrologSlurmctld and EpilogSlurmctld config.
//
// https://slurm.schedmd.com/slurm.conf.html#OPT_PrologSlurmctld
// https://slurm.schedmd.com/slurm.conf.html#OPT_EpilogSlurmctld
// https://slurm.schedmd.com/slurm.conf.html#SECTION_PROLOG-AND-EPILOG-SCRIPTS
func buildPrologEpilogSlurmctldConf(prologSlurmctldScripts, epilogSlurmctldScripts []string) string {
	conf := config.NewBuilder()

	sort.Strings(prologSlurmctldScripts)
	sort.Strings(epilogSlurmctldScripts)
	for _, filename := range prologSlurmctldScripts {
		scriptPath := path.Join(common.SlurmEtcDir, filename)
		conf.AddProperty(config.NewProperty("PrologSlurmctld", scriptPath))
	}
	for _, filename := range epilogSlurmctldScripts {
		scriptPath := path.Join(common.SlurmEtcDir, filename)
		conf.AddProperty(config.NewProperty("EpilogSlurmctld", scriptPath))
	}

	return conf.WithFinalNewline(false).Build()
}

// buildPrologEpilogConf() returns a slurm.conf snippet containing Prolog and Epilog config.
//
// https://slurm.schedmd.com/slurm.conf.html#OPT_Prolog
// https://slurm.schedmd.com/slurm.conf.html#OPT_Epilog
// https://slurm.schedmd.com/slurm.conf.html#SECTION_PROLOG-AND-EPILOG-SCRIPTS
func buildPrologEpilogConf(prologScripts, epilogScripts []string) string {
	conf := config.NewBuilder()

	sort.Strings(prologScripts)
	sort.Strings(epilogScripts)
	for _, filename := range prologScripts {
		conf.AddProperty(config.NewProperty("Prolog", filename))
	}
	for _, filename := range epilogScripts {
		conf.AddProperty(config.NewProperty("Epilog", filename))
	}

	return conf.WithFinalNewline(false).Build()
}

// nodeSetName returns the effective slurm.conf name for a NodeSet.
// The name is derived from the hostname if set, otherwise from the CR name.
func nodeSetName(nodeset *slinkyv1beta1.NodeSet) string {
	name := nodeset.Name
	if hostname := nodeset.Spec.Template.PodSpecWrapper.Hostname; hostname != "" {
		name = strings.Trim(hostname, "-")
	}
	return name
}

// nodeSetNames returns the set of effective slurm.conf names for all NodeSets.
func nodeSetNames(nodesetList *slinkyv1beta1.NodeSetList) map[string]struct{} {
	names := make(map[string]struct{}, len(nodesetList.Items))
	for i := range nodesetList.Items {
		names[nodeSetName(&nodesetList.Items[i])] = struct{}{}
	}
	return names
}

// filterExtraConfPartitions removes PartitionName lines from extraConf whose
// Nodes reference NodeSets that are not present in knownNames. This prevents
// generating an invalid slurm.conf when NodeSet CRs have not yet been created
// (e.g. during initial Helm install where the Controller CR is applied before
// NodeSet CRs).
func filterExtraConfPartitions(extraConf string, knownNames map[string]struct{}) string {
	if extraConf == "" {
		return ""
	}
	lines := strings.Split(extraConf, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if isPartitionLine(line) && !partitionNodesKnown(line, knownNames) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// isPartitionLine reports whether line is a Slurm PartitionName= directive.
func isPartitionLine(line string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "partitionname=")
}

// partitionNodesKnown reports whether all node names in the Nodes= field of a
// partition line are present in knownNames.
func partitionNodesKnown(line string, knownNames map[string]struct{}) bool {
	for _, field := range strings.Fields(line) {
		if !strings.HasPrefix(strings.ToLower(field), "nodes=") {
			continue
		}
		nodesValue := field[len("nodes="):]
		for _, name := range strings.Split(nodesValue, ",") {
			name = strings.TrimSpace(name)
			if name == "" || strings.EqualFold(name, "ALL") {
				continue
			}
			if _, ok := knownNames[name]; !ok {
				return false
			}
		}
		return true
	}
	return true
}

// buildNodeSetConf() returns a slurm.conf snippet containing NodeSets and their Partitions.
//
// https://slurm.schedmd.com/slurm.conf.html#SECTION_NODESET-CONFIGURATION
// https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION
func buildNodeSetConf(nodesetList *slinkyv1beta1.NodeSetList) string {
	conf := config.NewBuilder()

	sort.Slice(nodesetList.Items, func(i, j int) bool {
		return nodesetList.Items[i].Name < nodesetList.Items[j].Name
	})
	for _, nodeset := range nodesetList.Items {
		name := nodeSetName(&nodeset)
		nodesetLine := []string{
			fmt.Sprintf("NodeSet=%v", name),
			fmt.Sprintf("Feature=%v", name),
		}
		nodesetLineRendered := strings.Join(nodesetLine, " ")
		conf.AddProperty(config.NewPropertyRaw(nodesetLineRendered))
		partition := nodeset.Spec.Partition
		if !partition.Enabled {
			continue
		}
		partitionLine := []string{
			fmt.Sprintf("PartitionName=%v", name),
			fmt.Sprintf("Nodes=%v", name),
			partition.Config,
		}
		partitionLineRendered := strings.Join(partitionLine, " ")
		conf.AddProperty(config.NewPropertyRaw(partitionLineRendered))
	}

	return conf.WithFinalNewline(false).Build()
}

// https://slurm.schedmd.com/cgroup.conf.html
func buildCgroupConf() string {
	conf := config.NewBuilder()

	conf.AddProperty(config.NewProperty("CgroupPlugin", "cgroup/v2"))
	conf.AddProperty(config.NewProperty("IgnoreSystemd", "yes"))

	return conf.Build()
}

func isCgroupEnabled(cgroupConf string) bool {
	r := regexp.MustCompile(`(?im)^CgroupPlugin=disabled`)
	found := r.FindStringSubmatch(cgroupConf)
	return len(found) == 0
}

// https://slurm.schedmd.com/gres.conf.html
func buildGresConf() string {
	conf := config.NewBuilder()

	conf.AddProperty(config.NewProperty("AutoDetect", "nvidia"))

	return conf.Build()
}

// BuildControllerConfigExternal returns a minimal slurm.conf for slurmrestd (lacks configless).
func (b *ControllerBuilder) BuildControllerConfigExternal(controller *slinkyv1beta1.Controller) (*corev1.ConfigMap, error) {
	ctx := context.TODO()

	var accounting *slinkyv1beta1.Accounting
	if controller.Spec.AccountingRef != nil {
		var err error
		accounting, err = b.refResolver.GetAccounting(ctx, *controller.Spec.AccountingRef)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	opts := common.ConfigMapOpts{
		Key:      controller.ConfigKey(),
		Metadata: controller.Spec.Template.Metadata,
		Data: map[string]string{
			SlurmConfFile: buildSlurmConfMinimal(controller, accounting),
		},
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build())

	return b.CommonBuilder.BuildConfigMap(opts, controller)
}

// https://slurm.schedmd.com/slurm.conf.html
func buildSlurmConfMinimal(
	controller *slinkyv1beta1.Controller,
	accounting *slinkyv1beta1.Accounting,
) string {
	conf := config.NewBuilder()

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### GENERAL ###"))
	conf.AddProperty(config.NewProperty("ClusterName", controller.ClusterName()))
	conf.AddProperty(config.NewProperty("SlurmUser", common.SlurmUser))
	conf.AddProperty(config.NewProperty("SlurmctldHost", controller.PrimaryName()))
	conf.AddProperty(config.NewProperty("SlurmctldPort", common.SlurmctldPort))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### PLUGINS & PARAMETERS ###"))
	conf.AddProperty(config.NewProperty("AuthType", common.AuthType))
	conf.AddProperty(config.NewProperty("CredType", common.CredType))
	conf.AddProperty(config.NewProperty("AuthAltTypes", common.AuthAltTypes))

	conf.AddProperty(config.NewPropertyRaw("#"))
	conf.AddProperty(config.NewPropertyRaw("### ACCOUNTING ###"))
	if accounting != nil {
		conf.AddProperty(config.NewProperty("AccountingStorageType", "accounting_storage/slurmdbd"))
		conf.AddProperty(config.NewProperty("AccountingStorageHost", accounting.ServiceKey().Name))
		conf.AddProperty(config.NewProperty("AccountingStoragePort", common.SlurmdbdPort))
	} else {
		conf.AddProperty(config.NewProperty("AccountingStorageType", "accounting_storage/none"))
	}

	return conf.Build()
}
