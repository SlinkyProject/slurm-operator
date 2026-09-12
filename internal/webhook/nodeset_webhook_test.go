// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func TestPreferKubernetesNodeNameAdmission(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(*slinkyv1beta1.NodeSet)
		wantErr string
	}{
		{name: "valid"},
		{name: "allows unpinning", mutate: func(ns *slinkyv1beta1.NodeSet) { ns.Spec.PinToNode = false }},
		{name: "allows oversubscription", mutate: func(ns *slinkyv1beta1.NodeSet) { ns.Spec.OversubscribeNode = true }},
		{name: "accepts daemonset", mutate: func(ns *slinkyv1beta1.NodeSet) { ns.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset }},
		{name: "unpinned accepts custom arguments", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.PinToNode = false
			ns.Spec.Slurmd.Args = []string{"-Nother"}
		}},
		{name: "oversubscribed accepts custom arguments", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.OversubscribeNode = true
			ns.Spec.Slurmd.Args = []string{"-Nother"}
		}},
		{name: "rejects name argument", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.Slurmd.Args = []string{"-Nother"}
		}, wantErr: "slurmd.args must not override -N"},
		{name: "rejects reserved env", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.Slurmd.Env = []corev1.EnvVar{{Name: "SLURM_NODE_NAME", Value: "other"}}
		}, wantErr: "slurmd.env SLURM_NODE_NAME is reserved"},
		{name: "rejects reserved options env", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.Slurmd.Env = []corev1.EnvVar{{Name: "SLURMD_OPTIONS", Value: "-Nother"}}
		}, wantErr: "slurmd.env SLURMD_OPTIONS is reserved"},
		{name: "rejects command", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.Slurmd.Command = []string{"custom"}
		}, wantErr: "slurmd.command must not override the entrypoint"},
		{name: "rejects mode label", mutate: func(ns *slinkyv1beta1.NodeSet) {
			ns.Spec.Template.Metadata.Labels = map[string]string{slinkyv1beta1.LabelNodeSetSlurmNodeNameMode: "KubernetesNode"}
		}, wantErr: "the Slurm node naming mode Pod label is reserved"},
	} {
		t.Run(test.name, func(t *testing.T) {
			nodeset := &slinkyv1beta1.NodeSet{Spec: slinkyv1beta1.NodeSetSpec{
				ControllerRef:            corev1.LocalObjectReference{Name: "slurm"},
				ScalingMode:              slinkyv1beta1.ScalingModeStatefulset,
				PinToNode:                true,
				PreferKubernetesNodeName: true,
			}}
			old := nodeset.DeepCopy()
			if test.mutate != nil {
				test.mutate(nodeset)
			}
			webhook := &NodeSetWebhook{}
			createWarnings, createErr := webhook.ValidateCreate(context.Background(), nodeset)
			updateWarnings, updateErr := webhook.ValidateUpdate(context.Background(), old, nodeset)
			require.Empty(t, createWarnings)
			require.Empty(t, updateWarnings)
			if test.wantErr != "" {
				require.ErrorContains(t, createErr, test.wantErr)
				require.ErrorContains(t, updateErr, test.wantErr)
			} else {
				require.NoError(t, createErr)
				require.NoError(t, updateErr)
				warnings, err := webhook.ValidateUpdate(context.Background(), nodeset, old)
				require.NoError(t, err)
				require.Empty(t, warnings)
			}
		})
	}
}

func TestPreferKubernetesNodeNameImmutable(t *testing.T) {
	for _, oldPreference := range []bool{false, true} {
		for _, newPreference := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t to %t", oldPreference, newPreference), func(t *testing.T) {
				old := &slinkyv1beta1.NodeSet{Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{Name: "slurm"},
					ScalingMode:   slinkyv1beta1.ScalingModeStatefulset,
					PinToNode:     true, PreferKubernetesNodeName: oldPreference,
				}}
				updated := old.DeepCopy()
				updated.Spec.PreferKubernetesNodeName = newPreference
				updated.Spec.Replicas = ptr.To[int32](2)
				warns, err := (&NodeSetWebhook{}).ValidateUpdate(context.Background(), old, updated)
				if oldPreference != newPreference {
					require.ErrorContains(t, err, "preferKubernetesNodeName is immutable")
				} else {
					require.NoError(t, err)
				}
				require.Empty(t, warns)
			})
		}
	}
}

var _ = Describe("NodeSet Webhook", func() {
	It("resolves an autoscaled worker name when its Node appears and preserves it after binding", func(ctx SpecContext) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "binding-worker.example.com", Annotations: map[string]string{
			slinkyv1beta1.AnnotationNodeHostnameOverride: "gpu-01",
		}}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-worker-0", Namespace: "default", Labels: map[string]string{
				labels.AppLabel: labels.WorkerApp,
				slinkyv1beta1.LabelNodeSetSlurmNodeNameMode: string(slinkyv1beta1.SlurmNodeNameModeKubernetesNode),
			}},
			Spec: corev1.PodSpec{Hostname: "binding-worker-0", Containers: []corev1.Container{{
				Name: "slurmd", Image: "slurmd", Args: []string{"-Z", "-N", "$(SLURM_NODE_NAME)"},
				Env: []corev1.EnvVar{{Name: "SLURM_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['" + slinkyv1beta1.LabelNodeSetPodHostname + "']"},
				}}},
			}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) { Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed()) })
		Expect(pod.Spec.NodeName).To(BeEmpty())
		Expect(nodesetutils.GetSlurmNodeName(pod)).To(BeEmpty())
		binding := &corev1.Binding{ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace}, Target: corev1.ObjectReference{Kind: "Node", Name: node.Name}}
		Expect(k8sClient.SubResource("binding").Create(ctx, pod, binding)).NotTo(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.NodeName).To(BeEmpty())
		Expect(nodesetutils.GetSlurmNodeName(pod)).To(BeEmpty())
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) { Expect(k8sClient.Delete(ctx, node)).To(Succeed()) })
		Expect(k8sClient.SubResource("binding").Create(ctx, pod, binding)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.NodeName).To(Equal(node.Name))
		Expect(pod.Spec.Hostname).To(Equal("binding-worker-0"))
		Expect(pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname]).To(Equal("gpu-01"))
		Expect(nodesetutils.GetSlurmNodeName(pod)).To(Equal("gpu-01"))
		node.Annotations[slinkyv1beta1.AnnotationNodeHostnameOverride] = "gpu-02"
		Expect(k8sClient.Update(ctx, node)).To(Succeed())
		Expect(k8sClient.SubResource("binding").Create(ctx, pod, binding)).NotTo(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(nodesetutils.GetSlurmNodeName(pod)).To(Equal("gpu-01"))
	})

	Context("Slurm naming schema", func() {
		It("defaults null to false in either scaling mode", func(ctx SpecContext) {
			for _, scalingMode := range []string{"StatefulSet", "DaemonSet"} {
				nodeset := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": slinkyv1beta1.GroupVersion.String(),
					"kind":       "NodeSet",
					"metadata":   map[string]any{"generateName": "preferred-naming-", "namespace": "default"},
					"spec": map[string]any{
						"controllerRef":            map[string]any{"name": "slurm"},
						"preferKubernetesNodeName": nil, "scalingMode": scalingMode,
					},
				}}
				Expect(k8sClient.Create(ctx, nodeset)).To(Succeed())
				DeferCleanup(func(ctx SpecContext) { Expect(k8sClient.Delete(ctx, nodeset)).To(Succeed()) })
				prefer, _, err := unstructured.NestedBool(nodeset.Object, "spec", "preferKubernetesNodeName")
				Expect(err).NotTo(HaveOccurred())
				Expect(prefer).To(BeFalse())
				Expect(unstructured.SetNestedField(nodeset.Object, true, "spec", "preferKubernetesNodeName")).To(Succeed())
				Expect(apierrors.IsInvalid(k8sClient.Update(ctx, nodeset))).To(BeTrue())
				Expect(unstructured.SetNestedField(nodeset.Object, false, "spec", "preferKubernetesNodeName")).To(Succeed())
				Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
			}
		})
		It("preserves default naming and mutable placement on legacy objects", func(ctx SpecContext) {
			nodeset := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": slinkyv1beta1.GroupVersion.String(),
				"kind":       "NodeSet",
				"metadata":   map[string]any{"name": "naming-schema", "namespace": "default"},
				"spec":       map[string]any{"controllerRef": map[string]any{"name": "slurm"}},
			}}
			Expect(k8sClient.Create(ctx, nodeset)).To(Succeed())
			DeferCleanup(func(ctx SpecContext) { Expect(k8sClient.Delete(ctx, nodeset)).To(Succeed()) })
			prefer, _, err := unstructured.NestedBool(nodeset.Object, "spec", "preferKubernetesNodeName")
			Expect(err).NotTo(HaveOccurred())
			Expect(prefer).To(BeFalse())
			unstructured.RemoveNestedField(nodeset.Object, "spec", "preferKubernetesNodeName")
			Expect(unstructured.SetNestedField(nodeset.Object, true, "spec", "pinToNode")).To(Succeed())
			Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
			Expect(unstructured.SetNestedField(nodeset.Object, true, "spec", "oversubscribeNode")).To(Succeed())
			Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
			Expect(unstructured.SetNestedField(nodeset.Object, false, "spec", "pinToNode")).To(Succeed())
			Expect(unstructured.SetNestedField(nodeset.Object, false, "spec", "oversubscribeNode")).To(Succeed())
			Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
			Expect(unstructured.SetNestedField(nodeset.Object, true, "spec", "preferKubernetesNodeName")).To(Succeed())
			Expect(apierrors.IsInvalid(k8sClient.Update(ctx, nodeset))).To(BeTrue())
		})
		It("keeps the preference immutable without restricting placement changes", func(ctx SpecContext) {
			nodeset := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": slinkyv1beta1.GroupVersion.String(),
				"kind":       "NodeSet",
				"metadata":   map[string]any{"name": "node-naming-schema", "namespace": "default"},
				"spec": map[string]any{
					"controllerRef":            map[string]any{"name": "slurm"},
					"preferKubernetesNodeName": true, "pinToNode": true,
				},
			}}
			Expect(k8sClient.Create(ctx, nodeset)).To(Succeed())
			DeferCleanup(func(ctx SpecContext) { Expect(k8sClient.Delete(ctx, nodeset)).To(Succeed()) })
			Expect(unstructured.SetNestedField(nodeset.Object, int64(2), "spec", "replicas")).To(Succeed())
			Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
			for _, setting := range []string{"pinToNode", "oversubscribeNode"} {
				for _, value := range []bool{false, true, false, true} {
					Expect(unstructured.SetNestedField(nodeset.Object, value, "spec", setting)).To(Succeed())
					Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
				}
			}
			for _, remove := range []bool{false, true} {
				updated := nodeset.DeepCopy()
				if remove {
					unstructured.RemoveNestedField(updated.Object, "spec", "preferKubernetesNodeName")
				} else {
					Expect(unstructured.SetNestedField(updated.Object, false, "spec", "preferKubernetesNodeName")).To(Succeed())
				}
				err := k8sClient.Update(ctx, updated)
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("preferKubernetesNodeName is immutable"))
			}
		})
	})

	Context("When Creating a NodeSet with Validating Webhook", func() {
		It("Should deny if controllerRef.name is empty", func(ctx SpecContext) {
			nodeset := testutils.NewNodeset("test-nodeset", nil, 1)

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if maxUnavailable is 0", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromInt32(0))

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if maxUnavailable is 0%", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromString("0%"))

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if SSH is enabled without sssdConfRef", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Ssh.Enabled = true

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if template hostname is a valid generateName-style prefix", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Template.PodSpecWrapper.Hostname = "foo-"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny if template hostname contains a newline", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Template.PodSpecWrapper.Hostname = "evil\nSchedulerParameters=malicious"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if all required fields are provided", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny malformed extraConf", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.ExtraConf = "Weight10"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit valid extraConf", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.ExtraConf = "Feature=a Weight=5"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Updating a NodeSet with Validating Webhook", func() {
		It("Should reject changes to controllerRef", func(ctx SpecContext) {
			oldController := testutils.NewController("old-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", oldController, 1)

			newController := testutils.NewController("new-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newNodeSet := testutils.NewNodeset("test-nodeset", newController, 1)

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to volumeClaimTemplates", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)

			newNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			newNodeSet.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
			}

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if no immutable fields change", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			newNodeSet := testutils.NewNodeset("test-nodeset", controller, 2)

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Deleting a NodeSet with Validating Webhook", func() {
		It("Should admit a Delete", func(ctx SpecContext) {
			nodeset := testutils.NewNodeset("test-nodeset", nil, 1)

			_, err := nodeSetWebhook.ValidateDelete(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
