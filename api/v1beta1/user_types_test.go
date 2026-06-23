package v1beta1

import "testing"

func TestUser_GVK(t *testing.T) {
	if UserKind != "User" {
		t.Fatalf("UserKind = %q", UserKind)
	}
	if UserGVK.Kind != "User" {
		t.Fatalf("UserGVK.Kind = %q", UserGVK.Kind)
	}
}

func TestAdminLevelConstants(t *testing.T) {
	if AdminLevelNone != "None" || AdminLevelOperator != "Operator" || AdminLevelAdministrator != "Administrator" {
		t.Fatalf("unexpected AdminLevel values")
	}
}
