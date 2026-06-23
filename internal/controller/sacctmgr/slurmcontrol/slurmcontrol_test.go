// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"testing"

	"k8s.io/utils/ptr"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestBuildSlurmAccount(t *testing.T) {
	a := &slinkyv1beta1.Account{}
	a.Spec.AccountName = "research"
	a.Spec.Description = "Research team"
	a.Spec.Organization = "acme"
	got := buildSlurmAccount(a)
	if got.Name != "research" || got.Description != "Research team" || got.Organization != "acme" {
		t.Fatalf("unexpected account: %+v", got)
	}
}

func TestApplyLimits(t *testing.T) {
	assoc := slurmapi.V0044Assoc{Account: ptr.To("research"), User: ""}
	applyLimits(&assoc, slinkyv1beta1.AssociationLimits{
		Priority: ptr.To(int32(10)), QOS: []string{"normal"}, DefaultQOS: ptr.To("normal"),
		MaxJobs: ptr.To(int32(5)), GrpTRES: map[string]string{"cpu": "10", "gres/gpu": "4"},
	})
	if assoc.Account == nil || *assoc.Account != "research" {
		t.Fatalf("account clobbered: %+v", assoc.Account)
	}
	if assoc.Priority == nil || assoc.Priority.Number == nil || *assoc.Priority.Number != 10 {
		t.Fatalf("priority: %+v", assoc.Priority)
	}
	if assoc.Qos == nil || len(*assoc.Qos) != 1 || (*assoc.Qos)[0] != "normal" {
		t.Fatalf("qos: %+v", assoc.Qos)
	}
	if assoc.Default == nil || assoc.Default.Qos == nil || *assoc.Default.Qos != "normal" {
		t.Fatalf("default qos: %+v", assoc.Default)
	}
	if assoc.Max == nil || assoc.Max.Jobs == nil || assoc.Max.Jobs.Total == nil || assoc.Max.Jobs.Total.Number == nil || *assoc.Max.Jobs.Total.Number != 5 {
		t.Fatalf("max jobs: %+v", assoc.Max)
	}
	if assoc.Max.Tres == nil || assoc.Max.Tres.Group == nil || assoc.Max.Tres.Group.Active == nil || len(*assoc.Max.Tres.Group.Active) != 2 {
		t.Fatalf("grp tres: %+v", assoc.Max.Tres)
	}
}

func TestApplyLimitsFairshareParentSkipped(t *testing.T) {
	assoc := slurmapi.V0044Assoc{Account: ptr.To("research"), User: ""}
	applyLimits(&assoc, slinkyv1beta1.AssociationLimits{Fairshare: ptr.To("parent")})
	if assoc.SharesRaw != nil {
		t.Fatalf("expected shares_raw unset for non-numeric fairshare, got: %+v", assoc.SharesRaw)
	}
}

func TestApplyLimitsFairshareNumeric(t *testing.T) {
	assoc := slurmapi.V0044Assoc{Account: ptr.To("research"), User: ""}
	applyLimits(&assoc, slinkyv1beta1.AssociationLimits{Fairshare: ptr.To("100")})
	if assoc.SharesRaw == nil || *assoc.SharesRaw != 100 {
		t.Fatalf("shares_raw: %+v", assoc.SharesRaw)
	}
}

func TestTresList(t *testing.T) {
	got := tresList(map[string]string{"cpu": "10", "gres/gpu": "4"})
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	for _, entry := range got {
		switch entry["type"] {
		case "cpu":
			if entry["count"] != int64(10) {
				t.Fatalf("cpu count: %+v", entry)
			}
			if _, ok := entry["name"]; ok {
				t.Fatalf("cpu should not have name: %+v", entry)
			}
		case "gres":
			if entry["name"] != "gpu" {
				t.Fatalf("gres name: %+v", entry)
			}
			if entry["count"] != int64(4) {
				t.Fatalf("gres count: %+v", entry)
			}
		default:
			t.Fatalf("unexpected type: %+v", entry)
		}
	}
}

func TestNoVal(t *testing.T) {
	got := noVal(7)
	if got["set"] != true || got["number"] != int32(7) {
		t.Fatalf("noVal: %+v", got)
	}
}

func TestMapAdminLevel(t *testing.T) {
	cases := map[slinkyv1beta1.AdminLevel]slurmapi.V0044UserAdministratorLevel{
		slinkyv1beta1.AdminLevelNone:          slurmapi.V0044UserAdministratorLevelNone,
		slinkyv1beta1.AdminLevelOperator:      slurmapi.V0044UserAdministratorLevelOperator,
		slinkyv1beta1.AdminLevelAdministrator: slurmapi.V0044UserAdministratorLevelAdministrator,
		slinkyv1beta1.AdminLevel(""):          slurmapi.V0044UserAdministratorLevelNone,
	}
	for in, want := range cases {
		if got := mapAdminLevel(in); got != want {
			t.Fatalf("mapAdminLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildAccountAssoc(t *testing.T) {
	a := &slinkyv1beta1.Account{}
	a.Spec.AccountName = "research"
	a.Spec.ParentAccount = ptr.To("root")
	got := buildAccountAssoc(a)
	if got.Account == nil || *got.Account != "research" {
		t.Fatalf("account: %+v", got.Account)
	}
	if got.User != "" {
		t.Fatalf("expected empty user for account assoc, got: %q", got.User)
	}
	if got.ParentAccount == nil || *got.ParentAccount != "root" {
		t.Fatalf("parent account: %+v", got.ParentAccount)
	}
}

func TestBuildSlurmUser(t *testing.T) {
	u := &slinkyv1beta1.User{}
	u.Spec.UserName = "alice"
	u.Spec.AdminLevel = slinkyv1beta1.AdminLevelOperator
	u.Spec.DefaultAccount = "research"
	got := buildSlurmUser(u)
	if got.Name != "alice" {
		t.Fatalf("name: %+v", got.Name)
	}
	if got.AdministratorLevel == nil || len(*got.AdministratorLevel) != 1 || (*got.AdministratorLevel)[0] != slurmapi.V0044UserAdministratorLevelOperator {
		t.Fatalf("admin level: %+v", got.AdministratorLevel)
	}
	if got.Default == nil || got.Default.Account == nil || *got.Default.Account != "research" {
		t.Fatalf("default account: %+v", got.Default)
	}
}

func TestBuildUserAssoc(t *testing.T) {
	u := &slinkyv1beta1.User{}
	u.Spec.UserName = "alice"
	got := buildUserAssoc(u, slinkyv1beta1.UserAssociation{
		Account:   "research",
		Partition: ptr.To("debug"),
	})
	if got.Account == nil || *got.Account != "research" {
		t.Fatalf("account: %+v", got.Account)
	}
	if got.User != "alice" {
		t.Fatalf("user: %q", got.User)
	}
	if got.Partition == nil || *got.Partition != "debug" {
		t.Fatalf("partition: %+v", got.Partition)
	}
}
