package v1beta1

import "testing"

func TestAccount_GVK(t *testing.T) {
	if AccountKind != "Account" {
		t.Fatalf("AccountKind = %q", AccountKind)
	}
	if AccountGVK.Kind != "Account" {
		t.Fatalf("AccountGVK.Kind = %q", AccountGVK.Kind)
	}
}

func TestDeletionPolicyConstants(t *testing.T) {
	if DeletionPolicyDelete != "Delete" || DeletionPolicyOrphan != "Orphan" {
		t.Fatalf("unexpected DeletionPolicy values")
	}
}
