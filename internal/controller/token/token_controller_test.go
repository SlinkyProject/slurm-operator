// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Token Controller", func() {
	Context("When reconciling a Token", func() {
		var name = testutils.GenerateResourceName(5)
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should successfully create create a token", func(ctx SpecContext) {
			By("Creating Token CR")
			createdToken := &slinkyv1beta1.Token{}
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, createdToken)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("When deleting a Token", func() {
		var name = testutils.GenerateResourceName(5)
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should successfully create create a token", func(ctx SpecContext) {
			By("Creating Token CR")
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

		It("Should skip sync when the Token is being deleted", func(ctx SpecContext) {
			By("Creating Token CR")
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			}).Should(Succeed())

			By("Waiting for Token child to be created")
			secretKey := token.SecretKey()
			secret := &corev1.Secret{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Token with foregroud propagation")
			Expect(k8sClient.Delete(ctx, token,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Deleting Secret child while Token is terminating")
			Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			By("Verifying Secret child is NOT recreated")
			testutils.ExpectNotRecreated(ctx, k8sClient, secret)

			By("Cleaning up: removing foregroundDeletion finalizer")
			Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			token.Finalizers = nil
			Expect(k8sClient.Update(ctx, token)).To(Succeed())
		})

	})

	Context("When owning the generated auth-token Secret", func() {
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			name := testutils.GenerateResourceName(5)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should set the Token CR as the controller owner", func(ctx SpecContext) {
			By("Waiting for the auth-token Secret to be created")
			// SecretKey() is the generated credential, distinct from JwtKey() (the signing key).
			authSecret := waitForSecret(ctx, token.SecretKey())

			By("Verifying the controller owner is the Token, not the JWT signing key Secret")
			owner := metav1.GetControllerOf(authSecret)
			Expect(owner).NotTo(BeNil())
			Expect(owner.Kind).To(Equal(slinkyv1beta1.TokenKind))
			Expect(owner.Name).To(Equal(token.Name))
		}, SpecTimeout(testutils.Timeout))

		It("Should recreate the auth-token Secret when it is deleted out-of-band", func(ctx SpecContext) {
			By("Waiting for the auth-token Secret to be created")
			authSecret := waitForSecret(ctx, token.SecretKey())
			originalUID := authSecret.UID

			By("Deleting the auth-token Secret out-of-band")
			Expect(k8sClient.Delete(ctx, authSecret)).To(Succeed())

			By("Verifying the Secret watch drives a reconcile that recreates it")
			Eventually(func(g Gomega) {
				recreated := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, token.SecretKey(), recreated)).To(Succeed())
				g.Expect(recreated.UID).NotTo(Equal(originalUID))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("When adopting an auth-token Secret owned by the JWT signing key", func() {
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		// Refresh is disabled so the Secret is created immutable, which is the
		// case the normal sync path skips entirely.
		BeforeEach(func() {
			name := testutils.GenerateResourceName(5)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			token.Spec.Refresh = ptr.To(false)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should repoint the controller owner at the Token", func(ctx SpecContext) {
			By("Waiting for the immutable auth-token Secret to be created")
			authSecret := waitForSecret(ctx, token.SecretKey())
			Expect(ptr.Deref(authSecret.Immutable, false)).To(BeTrue())

			By("Rewriting the owner to the JWT signing key, as older operators did")
			staleOwner := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, token.JwtKey(), staleOwner)).To(Succeed())
			Expect(objectutils.PatchObject(k8sClient, ctx, authSecret, func(o *corev1.Secret) error {
				o.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(staleOwner, corev1.SchemeGroupVersion.WithKind("Secret")),
				}
				return nil
			})).To(Succeed())

			By("Triggering a reconcile, as an operator restart would")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(token), token)).To(Succeed())
			token.Annotations = structutils.MergeMaps(token.Annotations, map[string]string{
				"test.slinky.slurm.net/trigger": "adopt",
			})
			Expect(k8sClient.Update(ctx, token)).To(Succeed())

			By("Verifying the Secret is adopted by the Token")
			Eventually(func(g Gomega) {
				adopted := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, token.SecretKey(), adopted)).To(Succeed())
				owner := metav1.GetControllerOf(adopted)
				g.Expect(owner).NotTo(BeNil())
				g.Expect(owner.Kind).To(Equal(slinkyv1beta1.TokenKind))
				g.Expect(owner.UID).To(Equal(token.UID))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

	})

	Context("When the target Secret is owned by an unrelated resource", func() {
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret
		var unrelated *corev1.Secret

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, token)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, unrelated)
		})

		It("Should not take ownership of it", func(ctx SpecContext) {
			name := testutils.GenerateResourceName(5)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())

			By("Pre-creating the target Secret owned by an unrelated Secret")
			unrelated = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-unrelated",
					Namespace: token.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, unrelated)).To(Succeed())

			squatter := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      token.SecretKey().Name,
					Namespace: token.SecretKey().Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(unrelated, corev1.SchemeGroupVersion.WithKind("Secret")),
					},
				},
			}
			Expect(k8sClient.Create(ctx, squatter)).To(Succeed())

			By("Creating the Token that targets it")
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(token), token)).To(Succeed())

			By("Verifying the owner is never rewritten")
			Consistently(func(g Gomega) {
				current := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, token.SecretKey(), current)).To(Succeed())
				owner := metav1.GetControllerOf(current)
				g.Expect(owner).NotTo(BeNil())
				g.Expect(owner.UID).To(Equal(unrelated.UID))
			}, 5*time.Second, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})

func waitForSecret(ctx SpecContext, key types.NamespacedName) *corev1.Secret {
	GinkgoHelper()
	secret := &corev1.Secret{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, secret)).To(Succeed())
	}, testutils.Timeout, testutils.Interval).Should(Succeed())
	return secret
}
