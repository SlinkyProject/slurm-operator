// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/SlinkyProject/slurm-client/api/v0043"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func newFakeClientList(interceptorFuncs interceptor.Funcs, initObjLists ...object.ObjectList) slurmclient.Client {
	updateFn := func(_ context.Context, obj object.Object, req any, opts ...slurmclient.UpdateOption) error {
		switch o := obj.(type) {
		case *slurmtypes.V0043Node:
			r, ok := req.(api.V0043UpdateNodeMsg)
			if !ok {
				return errors.New("failed to cast request object")
			}
			stateSet := set.New(ptr.Deref(o.State, []api.V0043NodeState{})...)
			statesReq := ptr.Deref(r.State, []api.V0043UpdateNodeMsgState{})
			for _, stateReq := range statesReq {
				switch stateReq {
				case api.V0043UpdateNodeMsgStateUNDRAIN:
					stateSet.Delete(api.V0043NodeStateDRAIN)
				default:
					stateSet.Insert(api.V0043NodeState(stateReq))
				}
			}
			o.State = ptr.To(stateSet.UnsortedList())
			o.Comment = r.Comment
			o.Reason = r.Reason
		default:
			return errors.New("failed to cast slurm object")
		}
		return nil
	}

	return fake.NewClientBuilder().
		WithUpdateFn(updateFn).
		WithLists(initObjLists...).
		WithInterceptorFuncs(interceptorFuncs).
		Build()
}

var _ = Describe("Slurm NodeSet", func() {
	Context("When creating NodeSet", func() {
		var name = testutils.GenerateResourceName(5)
		var nodeset *slinkyv1beta1.NodeSet
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			nodeset = testutils.NewNodeset(name, controller, 0)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, nodeset.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, nodeset)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should successfully create create a nodeset", func(ctx SpecContext) {
			By("Creating NodeSet CR")
			createdNodeset := &slinkyv1beta1.NodeSet{}
			nodesetKey := k8sclient.ObjectKeyFromObject(nodeset)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nodesetKey, createdNodeset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Waiting for N replicas")
			podList := &corev1.PodList{}
			optsList := &k8sclient.ListOptions{
				Namespace: nodeset.Namespace,
			}
			replicas := int(ptr.Deref(nodeset.Spec.Replicas, 0))
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, podList, optsList)).To(Succeed())
				g.Expect(len(podList.Items)).Should(Equal(replicas))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

		}, SpecTimeout(testutils.Timeout))
	})

	Context("Scaling unhealthy replicas", func() {
		var name = testutils.GenerateResourceName(5)
		var nodeset *slinkyv1beta1.NodeSet
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			nodeset = testutils.NewNodeset(name, controller, 0)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, nodeset.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, nodeset)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should scale replicas", func(ctx SpecContext) {
			nodesetKey := k8sclient.ObjectKeyFromObject(nodeset)
			controllerKey := k8sclient.ObjectKeyFromObject(controller)

			By("Waiting for N replicas")
			podList := &corev1.PodList{}
			optsList := &k8sclient.ListOptions{
				Namespace: nodeset.Namespace,
			}
			replicas := int(ptr.Deref(nodeset.Spec.Replicas, 0))
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, podList, optsList)).To(Succeed())
				g.Expect(len(podList.Items)).Should(Equal(replicas))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			clientMap.Add(controllerKey, newFakeClientList(interceptor.Funcs{}))

			By("Scaling in replicas")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nodesetKey, nodeset)).To(Succeed())
				nodeset.Spec.Replicas = ptr.To[int32](0)
				g.Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
				replicas := int(ptr.Deref(nodeset.Spec.Replicas, 0))
				g.Expect(replicas).Should(Equal(0))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying pods were deleted")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, podList, optsList)).To(Succeed())
				g.Expect(len(podList.Items)).Should(Equal(0))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("Scaling healthy replicas", func() {
		var name = testutils.GenerateResourceName(5)
		var nodeset *slinkyv1beta1.NodeSet
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			nodeset = testutils.NewNodeset(name, controller, 0)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, nodeset.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, nodeset)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should scale replicas", func(ctx SpecContext) {
			nodesetKey := k8sclient.ObjectKeyFromObject(nodeset)
			controllerKey := k8sclient.ObjectKeyFromObject(controller)

			By("Waiting for N replicas")
			podList := &corev1.PodList{}
			optsList := &k8sclient.ListOptions{
				Namespace: nodeset.Namespace,
			}
			replicas := int(ptr.Deref(nodeset.Spec.Replicas, 0))
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, podList, optsList)).To(Succeed())
				g.Expect(len(podList.Items)).Should(Equal(replicas))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Simulating Slurm functionality")
			slurmNodes := make([]slurmtypes.V0043Node, 0)
			for _, pod := range podList.Items {
				// Register Slurm node for pod
				node := slurmtypes.V0043Node{
					V0043Node: api.V0043Node{
						Name:  ptr.To(nodesetutils.GetNodeName(&pod)),
						State: ptr.To([]api.V0043NodeState{api.V0043NodeStateIDLE}),
					},
				}
				slurmNodes = append(slurmNodes, node)
			}
			clientMap.Add(controllerKey,
				newFakeClientList(interceptor.Funcs{}, &slurmtypes.V0043NodeList{
					Items: slurmNodes,
				}),
			)

			By("Simulating Kubernetes functionality")
			for _, pod := range podList.Items {
				// Mark pod as being healthy
				pod.Status.Phase = corev1.PodRunning
				podCond := corev1.PodCondition{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}
				pod.Status.Conditions = append(pod.Status.Conditions, podCond)
				Expect(k8sClient.Status().Update(ctx, &pod)).To(Succeed())
			}

			By("Scaling in replicas")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, nodesetKey, nodeset)).To(Succeed())
				nodeset.Spec.Replicas = ptr.To[int32](0)
				g.Expect(k8sClient.Update(ctx, nodeset)).To(Succeed())
				replicas := int(ptr.Deref(nodeset.Spec.Replicas, 0))
				g.Expect(replicas).Should(Equal(0))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying Slurm nodes were drained first")
			slurmClient := clientMap.Get(controllerKey)
			Eventually(func(g Gomega) {
				slurmNodes := &slurmtypes.V0043NodeList{}
				g.Expect(slurmClient.List(ctx, slurmNodes)).To(Succeed())
				for _, node := range slurmNodes.Items {
					g.Expect(node.GetStateAsSet().Has(api.V0043NodeStateDRAIN)).Should(BeTrue())
				}
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying pods were deleted")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, podList, optsList)).To(Succeed())
				g.Expect(len(podList.Items)).Should(Equal(0))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Simulating Slurm nodes being unregistered")
			clientMap.Add(controllerKey, newFakeClientList(interceptor.Funcs{}))
		}, SpecTimeout(testutils.Timeout))
	})
})
