// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		MaxJobs: ptr.To(int32(5)), MaxSubmitJobs: ptr.To(int32(9)),
		GrpTRES: map[string]string{"cpu": "10", "gres/gpu": "4"},
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
	if assoc.Max == nil || assoc.Max.Jobs == nil || assoc.Max.Jobs.Active == nil || assoc.Max.Jobs.Active.Number == nil || *assoc.Max.Jobs.Active.Number != 5 {
		t.Fatalf("max jobs (active): %+v", assoc.Max)
	}
	if assoc.Max.Jobs.Total == nil || assoc.Max.Jobs.Total.Number == nil || *assoc.Max.Jobs.Total.Number != 9 {
		t.Fatalf("max submit jobs (total): %+v", assoc.Max.Jobs)
	}
	if assoc.Max.Tres == nil || assoc.Max.Tres.Total == nil || len(*assoc.Max.Tres.Total) != 2 {
		t.Fatalf("grp tres: %+v", assoc.Max.Tres)
	}
}

// TestApplyLimitsAllFields exercises every AssociationLimits field and asserts
// the exact v0044 association schema path each one maps to.
func TestApplyLimitsAllFields(t *testing.T) {
	assoc := slurmapi.V0044Assoc{
		Account:   ptr.To("research"),
		User:      "alice",
		Partition: ptr.To("debug"),
	}
	applyLimits(&assoc, slinkyv1beta1.AssociationLimits{
		Fairshare:     ptr.To("100"),
		Priority:      ptr.To(int32(10)),
		QOS:           []string{"normal", "high"},
		DefaultQOS:    ptr.To("normal"),
		GrpTRES:       map[string]string{"cpu": "100", "gres/gpu": "8"},
		GrpJobs:       ptr.To(int32(20)),
		GrpSubmitJobs: ptr.To(int32(40)),
		GrpWall:       &metav1.Duration{Duration: time.Hour},
		MaxJobs:       ptr.To(int32(5)),
		MaxSubmitJobs: ptr.To(int32(9)),
		MaxTRESPerJob: map[string]string{"cpu": "4"},
		MaxWallPerJob: &metav1.Duration{Duration: 30 * time.Minute},
	})

	// Identity fields must be preserved (not clobbered by the merge).
	if assoc.Account == nil || *assoc.Account != "research" || assoc.User != "alice" ||
		assoc.Partition == nil || *assoc.Partition != "debug" {
		t.Fatalf("identity clobbered: %+v", assoc)
	}

	// Fairshare (numeric) -> shares_raw
	if assoc.SharesRaw == nil || *assoc.SharesRaw != 100 {
		t.Fatalf("shares_raw: %+v", assoc.SharesRaw)
	}
	// Priority -> priority
	if assoc.Priority == nil || assoc.Priority.Number == nil || *assoc.Priority.Number != 10 {
		t.Fatalf("priority: %+v", assoc.Priority)
	}
	// QOS -> qos
	if assoc.Qos == nil || len(*assoc.Qos) != 2 || (*assoc.Qos)[0] != "normal" || (*assoc.Qos)[1] != "high" {
		t.Fatalf("qos: %+v", assoc.Qos)
	}
	// DefaultQOS -> default.qos
	if assoc.Default == nil || assoc.Default.Qos == nil || *assoc.Default.Qos != "normal" {
		t.Fatalf("default qos: %+v", assoc.Default)
	}

	if assoc.Max == nil || assoc.Max.Jobs == nil {
		t.Fatalf("max.jobs nil: %+v", assoc.Max)
	}
	// MaxJobs -> max.jobs.active
	if assoc.Max.Jobs.Active == nil || assoc.Max.Jobs.Active.Number == nil || *assoc.Max.Jobs.Active.Number != 5 {
		t.Fatalf("max jobs (active): %+v", assoc.Max.Jobs)
	}
	// MaxSubmitJobs -> max.jobs.total
	if assoc.Max.Jobs.Total == nil || assoc.Max.Jobs.Total.Number == nil || *assoc.Max.Jobs.Total.Number != 9 {
		t.Fatalf("max submit jobs (total): %+v", assoc.Max.Jobs)
	}
	// MaxWallPerJob -> max.jobs.per.wall_clock (in minutes)
	if assoc.Max.Jobs.Per == nil || assoc.Max.Jobs.Per.WallClock == nil ||
		assoc.Max.Jobs.Per.WallClock.Number == nil || *assoc.Max.Jobs.Per.WallClock.Number != 30 {
		t.Fatalf("max wall per job: %+v", assoc.Max.Jobs.Per)
	}
	// GrpJobs -> max.jobs.per.count
	if assoc.Max.Jobs.Per.Count == nil || assoc.Max.Jobs.Per.Count.Number == nil || *assoc.Max.Jobs.Per.Count.Number != 20 {
		t.Fatalf("grp jobs (per.count): %+v", assoc.Max.Jobs.Per)
	}
	// GrpSubmitJobs -> max.jobs.per.submitted
	if assoc.Max.Jobs.Per.Submitted == nil || assoc.Max.Jobs.Per.Submitted.Number == nil || *assoc.Max.Jobs.Per.Submitted.Number != 40 {
		t.Fatalf("grp submit jobs (per.submitted): %+v", assoc.Max.Jobs.Per)
	}
	// GrpWall -> max.per.account.wall_clock (in minutes)
	if assoc.Max.Per == nil || assoc.Max.Per.Account == nil || assoc.Max.Per.Account.WallClock == nil ||
		assoc.Max.Per.Account.WallClock.Number == nil || *assoc.Max.Per.Account.WallClock.Number != 60 {
		t.Fatalf("grp wall (per.account.wall_clock): %+v", assoc.Max.Per)
	}

	// GrpTRES -> max.tres.total
	if assoc.Max.Tres == nil || assoc.Max.Tres.Total == nil {
		t.Fatalf("grp tres nil: %+v", assoc.Max.Tres)
	}
	if got := tresCount(t, *assoc.Max.Tres.Total, "cpu", ""); got != 100 {
		t.Fatalf("grp tres cpu = %d, want 100", got)
	}
	if got := tresCount(t, *assoc.Max.Tres.Total, "gres", "gpu"); got != 8 {
		t.Fatalf("grp tres gres/gpu = %d, want 8", got)
	}
	// MaxTRESPerJob -> max.tres.per.job
	if assoc.Max.Tres.Per == nil || assoc.Max.Tres.Per.Job == nil {
		t.Fatalf("max tres per job nil: %+v", assoc.Max.Tres)
	}
	if got := tresCount(t, *assoc.Max.Tres.Per.Job, "cpu", ""); got != 4 {
		t.Fatalf("max tres per job cpu = %d, want 4", got)
	}
}

// tresCount returns the count for the (type, name) TRES entry, or fails the
// test if it is missing. An empty name matches a generic (nameless) entry.
func tresCount(t *testing.T, list slurmapi.V0044TresList, typ, name string) int64 {
	t.Helper()
	for _, tr := range list {
		gotName := ""
		if tr.Name != nil {
			gotName = *tr.Name
		}
		if tr.Type == typ && gotName == name {
			if tr.Count == nil {
				t.Fatalf("tres %s/%s has nil count", typ, name)
			}
			return *tr.Count
		}
	}
	t.Fatalf("tres %s/%s not found in %+v", typ, name, list)
	return 0
}

func TestIgnoreNotModified(t *testing.T) {
	if err := ignoreNotModified(nil); err != nil {
		t.Fatalf("nil should stay nil, got: %v", err)
	}
	if err := ignoreNotModified(errors.New("Not Modified")); err != nil {
		t.Fatalf("Not Modified should be tolerated, got: %v", err)
	}
	if err := ignoreNotModified(errors.New("Internal Server Error")); err == nil {
		t.Fatalf("real error should not be tolerated")
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
	got := buildAccountAssoc(a, "slurm_slurm")
	if got.Account == nil || *got.Account != "research" {
		t.Fatalf("account: %+v", got.Account)
	}
	if got.Cluster == nil || *got.Cluster != "slurm_slurm" {
		t.Fatalf("cluster: %+v", got.Cluster)
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
	}, "slurm_slurm")
	if got.Account == nil || *got.Account != "research" {
		t.Fatalf("account: %+v", got.Account)
	}
	if got.Cluster == nil || *got.Cluster != "slurm_slurm" {
		t.Fatalf("cluster: %+v", got.Cluster)
	}
	if got.User != "alice" {
		t.Fatalf("user: %q", got.User)
	}
	if got.Partition == nil || *got.Partition != "debug" {
		t.Fatalf("partition: %+v", got.Partition)
	}
}
