package defaults

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetAccountDefaults(t *testing.T) {
	a := &slinkyv1beta1.Account{}
	a.Name = "research"
	SetAccountDefaults(a)
	if a.Spec.AccountName != "research" {
		t.Errorf("AccountName = %q, want research", a.Spec.AccountName)
	}
	if a.Spec.Description != "research" {
		t.Errorf("Description = %q, want research", a.Spec.Description)
	}
	if a.Spec.Organization != "root" {
		t.Errorf("Organization = %q, want root", a.Spec.Organization)
	}
	if a.Spec.ParentAccount == nil || *a.Spec.ParentAccount != "root" {
		t.Errorf("ParentAccount not defaulted to root")
	}
	if a.Spec.DeletionPolicy != slinkyv1beta1.DeletionPolicyDelete {
		t.Errorf("DeletionPolicy = %q, want Delete", a.Spec.DeletionPolicy)
	}
}

func TestSetAccountDefaultsPreservesExplicitDescription(t *testing.T) {
	a := &slinkyv1beta1.Account{}
	a.Name = "research"
	a.Spec.Description = "Research team"
	a.Spec.Organization = "acme"
	SetAccountDefaults(a)
	if a.Spec.Description != "Research team" {
		t.Errorf("Description = %q, want preserved 'Research team'", a.Spec.Description)
	}
	if a.Spec.Organization != "acme" {
		t.Errorf("Organization = %q, want preserved 'acme'", a.Spec.Organization)
	}
}
