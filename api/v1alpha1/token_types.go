// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TokenKind = "Token"
)

var (
	TokenGVK        = GroupVersion.WithKind(TokenKind)
	TokenAPIVersion = GroupVersion.String()
)

// TokenSpec defines the desired state of Token
type TokenSpec struct {
	// Slurm `auth/jwt` JWT HS256 key authentication.
	// +required
	JwtHs256KeyRef JwtSecretKeySelector `json:"jwtHs256KeyRef,omitzero"`

	// The username whom the token is created for.
	// +required
	Username string `json:"username,omitzero"`

	// The lifetime of the JWT before it expires.
	// +optional
	Lifetime *metav1.Duration `json:"lifetime,omitempty"`

	// Controls if the JWT will be rotated.
	// If set to false, then the secret will be created as immutable.
	// +optional
	// +default:=true
	Refresh bool `json:"refresh,omitzero"`

	// SecretRef describes how to create the secret containing the JWT.
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// TokenStatus defines the observed state of Token
type TokenStatus struct {
	// IssuedAt indicates the time when the JWT was issued.
	IssuedAt *metav1.Time `json:"issuedAt,omitempty"`

	// Represents the latest available observations of a Restapi's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:deprecatedversion:warning="v1alpha1 is deprecated, use v1beta1."
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tokens;jwt
// +kubebuilder:printcolumn:name="USER",type="string",JSONPath=".spec.username",description="The username issued to the JWT."
// +kubebuilder:printcolumn:name="IAT",type="date",JSONPath=".status.issuedAt",description="The JWT Issued At time."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Token is the Schema for the tokens API
type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TokenSpec   `json:"spec,omitempty"`
	Status TokenStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TokenList contains a list of Token
type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Token `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Token{}, &TokenList{})
}
