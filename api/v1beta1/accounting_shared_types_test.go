package v1beta1

import "testing"

func TestDeletionPolicyConstants(t *testing.T) {
	if DeletionPolicyDelete != "Delete" || DeletionPolicyOrphan != "Orphan" {
		t.Fatalf("unexpected DeletionPolicy values")
	}
	if AdminLevelNone != "None" || AdminLevelAdministrator != "Administrator" {
		t.Fatalf("unexpected AdminLevel values")
	}
}
