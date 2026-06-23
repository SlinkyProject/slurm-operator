package defaults

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetUserDefaults(t *testing.T) {
	u := &slinkyv1beta1.User{}
	u.Name = "alice"
	SetUserDefaults(u)
	if u.Spec.UserName != "alice" {
		t.Errorf("UserName = %q, want alice", u.Spec.UserName)
	}
	if u.Spec.AdminLevel != slinkyv1beta1.AdminLevelNone {
		t.Errorf("AdminLevel = %q, want None", u.Spec.AdminLevel)
	}
	if u.Spec.DeletionPolicy != slinkyv1beta1.DeletionPolicyDelete {
		t.Errorf("DeletionPolicy = %q, want Delete", u.Spec.DeletionPolicy)
	}
}
