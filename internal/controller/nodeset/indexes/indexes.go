// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package indexes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/confighash"
)

// NodeSetConfigRefsField indexes opted-in NodeSets by the Secrets/ConfigMaps
// they mount. Values are of the form "secret/<name>" or "configmap/<name>".
const NodeSetConfigRefsField = "nodeset.confighash.refs"

// Prefixes for NodeSetConfigRefsField index values.
const (
	SecretRefPrefix    = "secret/"
	ConfigMapRefPrefix = "configmap/"
)

type clientIndexer struct {
	obj   client.Object
	field string
	fn    client.IndexerFunc
}

var indexers = []clientIndexer{
	{
		obj:   &corev1.Pod{},
		field: "spec.nodeName",
		fn:    getPodNodeName,
	},
	{
		obj:   &slinkyv1beta1.NodeSet{},
		field: NodeSetConfigRefsField,
		fn:    getNodeSetConfigRefs,
	},
}

func getPodNodeName(o client.Object) []string {
	obj, ok := o.(runtime.Object)
	if !ok {
		return []string{}
	}
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return []string{}
	}
	return []string{pod.Spec.NodeName}
}

func getNodeSetConfigRefs(o client.Object) []string {
	ns, ok := o.(*slinkyv1beta1.NodeSet)
	if !ok {
		return nil
	}
	if ns.Annotations[slinkyv1beta1.AnnotationReloadOnChange] != "true" {
		return nil
	}
	secrets, configMaps := confighash.References(ns.Spec.Template.PodSpecWrapper.PodSpec)
	out := make([]string, 0, len(secrets)+len(configMaps))
	for _, name := range secrets {
		out = append(out, SecretRefPrefix+name)
	}
	for _, name := range configMaps {
		out = append(out, ConfigMapRefPrefix+name)
	}
	return out
}

func SetupWithManager(mgr ctrl.Manager) error {
	for _, indexer := range indexers {
		err := mgr.GetFieldIndexer().IndexField(context.Background(), indexer.obj, indexer.field, indexer.fn)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewFakeClientBuilderWithIndexes returns a client builder with the equivalent of addIndexers applied.
func NewFakeClientBuilderWithIndexes(initObjs ...runtime.Object) *fake.ClientBuilder {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(slinkyv1beta1.AddToScheme(s))
	cb := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(initObjs...)
	for _, indexer := range indexers {
		cb = cb.WithIndex(indexer.obj, indexer.field, indexer.fn)
	}
	return cb
}
