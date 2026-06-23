// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
)

// ErrNoClient is returned when no Slurm client is registered for the
// controllerRef referenced by an Account or User.
var ErrNoClient = errors.New("no slurm client available for controllerRef")

// AccountingControlInterface manages slurmdbd accounts, users, and their
// associations on behalf of the Account and User controllers.
type AccountingControlInterface interface {
	GetAccount(ctx context.Context, account *slinkyv1beta1.Account) (*slurmtypes.V0044Account, error)
	ApplyAccount(ctx context.Context, account *slinkyv1beta1.Account) error
	DeleteAccount(ctx context.Context, account *slinkyv1beta1.Account) error
	AccountExists(ctx context.Context, user *slinkyv1beta1.User, accountName string) (bool, error)
	GetUser(ctx context.Context, user *slinkyv1beta1.User) (*slurmtypes.V0044User, error)
	ApplyUser(ctx context.Context, user *slinkyv1beta1.User) error
	DeleteUser(ctx context.Context, user *slinkyv1beta1.User) error
}

// realAccountingControl is the default implementation of AccountingControlInterface.
type realAccountingControl struct {
	clientMap *clientmap.ClientMap

	// clusterNames caches the discovered Slurm cluster name per controllerRef.
	// slurmdbd associations are scoped to a cluster; an association POST that
	// omits the cluster is silently dropped, so the name must be set on every
	// association we create.
	mu           sync.Mutex
	clusterNames map[string]string
}

var _ AccountingControlInterface = &realAccountingControl{}

// NewAccountingControl returns an AccountingControlInterface backed by the given ClientMap.
func NewAccountingControl(clientMap *clientmap.ClientMap) AccountingControlInterface {
	return &realAccountingControl{
		clientMap:    clientMap,
		clusterNames: make(map[string]string),
	}
}

func (r *realAccountingControl) lookupClient(namespace, controllerName string) slurmclient.Client {
	key := ktypes.NamespacedName{
		Namespace: namespace,
		Name:      controllerName,
	}
	return r.clientMap.Get(key)
}

// clusterName discovers (and caches) the Slurm cluster name for the given
// controllerRef. The name is read from any existing association (the root
// association always exists once slurmdbd is up), since slurmdbd does not
// expose a dedicated cluster lookup through this client.
func (r *realAccountingControl) clusterName(ctx context.Context, namespace, controllerName string, c slurmclient.Client) (string, error) {
	cacheKey := namespace + "/" + controllerName

	r.mu.Lock()
	if name, ok := r.clusterNames[cacheKey]; ok {
		r.mu.Unlock()
		return name, nil
	}
	r.mu.Unlock()

	list := &slurmtypes.V0044AssocList{}
	if err := c.List(ctx, list); err != nil {
		return "", err
	}
	name := ""
	for i := range list.Items {
		if cn := ptr.Deref(list.Items[i].Cluster, ""); cn != "" {
			name = cn
			break
		}
	}
	if name == "" {
		return "", fmt.Errorf("unable to determine slurm cluster name for controllerRef %q", controllerName)
	}

	r.mu.Lock()
	r.clusterNames[cacheKey] = name
	r.mu.Unlock()
	return name, nil
}

// GetAccount implements AccountingControlInterface.
func (r *realAccountingControl) GetAccount(ctx context.Context, account *slinkyv1beta1.Account) (*slurmtypes.V0044Account, error) {
	c := r.lookupClient(account.Namespace, account.Spec.ControllerRef.Name)
	if c == nil {
		return nil, ErrNoClient
	}
	out := &slurmtypes.V0044Account{}
	if err := c.Get(ctx, slurmobject.ObjectKey(account.Spec.AccountName), out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyAccount upserts the Slurm account and its account-level association
// (which carries the parent and limits). POST is an upsert in slurmdbd.
func (r *realAccountingControl) ApplyAccount(ctx context.Context, account *slinkyv1beta1.Account) error {
	c := r.lookupClient(account.Namespace, account.Spec.ControllerRef.Name)
	if c == nil {
		return ErrNoClient
	}
	if err := ignoreNotModified(c.Create(ctx, &slurmtypes.V0044Account{}, buildSlurmAccount(account))); err != nil {
		return err
	}
	cluster, err := r.clusterName(ctx, account.Namespace, account.Spec.ControllerRef.Name, c)
	if err != nil {
		return err
	}
	return ignoreNotModified(c.Create(ctx, &slurmtypes.V0044Assoc{}, buildAccountAssoc(account, cluster)))
}

// DeleteAccount implements AccountingControlInterface, honoring the deletion policy.
func (r *realAccountingControl) DeleteAccount(ctx context.Context, account *slinkyv1beta1.Account) error {
	if account.Spec.DeletionPolicy == slinkyv1beta1.DeletionPolicyOrphan {
		return nil
	}
	c := r.lookupClient(account.Namespace, account.Spec.ControllerRef.Name)
	if c == nil {
		return ErrNoClient
	}
	obj := &slurmtypes.V0044Account{}
	obj.Name = account.Spec.AccountName
	if err := c.Delete(ctx, obj); err != nil && !tolerateError(err) {
		return err
	}
	return nil
}

// AccountExists reports whether the named account exists in the Slurm cluster
// targeted by the user's controllerRef.
func (r *realAccountingControl) AccountExists(ctx context.Context, user *slinkyv1beta1.User, accountName string) (bool, error) {
	c := r.lookupClient(user.Namespace, user.Spec.ControllerRef.Name)
	if c == nil {
		return false, ErrNoClient
	}
	out := &slurmtypes.V0044Account{}
	err := c.Get(ctx, slurmobject.ObjectKey(accountName), out)
	if err == nil {
		return true, nil
	}
	if tolerateError(err) {
		return false, nil
	}
	return false, err
}

// GetUser implements AccountingControlInterface.
func (r *realAccountingControl) GetUser(ctx context.Context, user *slinkyv1beta1.User) (*slurmtypes.V0044User, error) {
	c := r.lookupClient(user.Namespace, user.Spec.ControllerRef.Name)
	if c == nil {
		return nil, ErrNoClient
	}
	out := &slurmtypes.V0044User{}
	if err := c.Get(ctx, slurmobject.ObjectKey(user.Spec.UserName), out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyUser upserts the Slurm user and one association per membership.
func (r *realAccountingControl) ApplyUser(ctx context.Context, user *slinkyv1beta1.User) error {
	c := r.lookupClient(user.Namespace, user.Spec.ControllerRef.Name)
	if c == nil {
		return ErrNoClient
	}
	if err := ignoreNotModified(c.Create(ctx, &slurmtypes.V0044User{}, buildSlurmUser(user))); err != nil {
		return err
	}
	cluster, err := r.clusterName(ctx, user.Namespace, user.Spec.ControllerRef.Name, c)
	if err != nil {
		return err
	}
	for _, ua := range user.Spec.Associations {
		if err := ignoreNotModified(c.Create(ctx, &slurmtypes.V0044Assoc{}, buildUserAssoc(user, ua, cluster))); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUser implements AccountingControlInterface, honoring the deletion policy.
func (r *realAccountingControl) DeleteUser(ctx context.Context, user *slinkyv1beta1.User) error {
	if user.Spec.DeletionPolicy == slinkyv1beta1.DeletionPolicyOrphan {
		return nil
	}
	c := r.lookupClient(user.Namespace, user.Spec.ControllerRef.Name)
	if c == nil {
		return ErrNoClient
	}
	obj := &slurmtypes.V0044User{}
	obj.Name = user.Spec.UserName
	if err := c.Delete(ctx, obj); err != nil && !tolerateError(err) {
		return err
	}
	return nil
}

// buildSlurmAccount maps an Account CR to the slurmdbd account payload.
func buildSlurmAccount(account *slinkyv1beta1.Account) slurmapi.V0044Account {
	return slurmapi.V0044Account{
		Name:         account.Spec.AccountName,
		Description:  account.Spec.Description,
		Organization: account.Spec.Organization,
	}
}

// buildAccountAssoc maps an Account CR to its account-level association
// (empty user), carrying the parent account and limits. The cluster must be
// set or slurmdbd silently drops the association write.
func buildAccountAssoc(account *slinkyv1beta1.Account, cluster string) slurmapi.V0044Assoc {
	assoc := slurmapi.V0044Assoc{
		Cluster:       ptr.To(cluster),
		Account:       ptr.To(account.Spec.AccountName),
		User:          "",
		ParentAccount: account.Spec.ParentAccount,
	}
	applyLimits(&assoc, account.Spec.Limits)
	return assoc
}

// buildSlurmUser maps a User CR to the slurmdbd user payload.
func buildSlurmUser(user *slinkyv1beta1.User) slurmapi.V0044User {
	m := map[string]any{
		"name":                user.Spec.UserName,
		"administrator_level": []string{string(mapAdminLevel(user.Spec.AdminLevel))},
	}
	if user.Spec.DefaultAccount != "" {
		m["default"] = map[string]any{"account": user.Spec.DefaultAccount}
	}
	out := slurmapi.V0044User{}
	jsonMerge(m, &out)
	return out
}

// buildUserAssoc maps a single UserAssociation to a slurmdbd association. The
// cluster must be set or slurmdbd silently drops the association write.
func buildUserAssoc(user *slinkyv1beta1.User, ua slinkyv1beta1.UserAssociation, cluster string) slurmapi.V0044Assoc {
	assoc := slurmapi.V0044Assoc{
		Cluster:   ptr.To(cluster),
		Account:   ptr.To(ua.Account),
		User:      user.Spec.UserName,
		Partition: ua.Partition,
	}
	applyLimits(&assoc, ua.Limits)
	return assoc
}

// mapAdminLevel maps the CR AdminLevel to the Slurm administrator level.
func mapAdminLevel(level slinkyv1beta1.AdminLevel) slurmapi.V0044UserAdministratorLevel {
	switch level {
	case slinkyv1beta1.AdminLevelOperator:
		return slurmapi.V0044UserAdministratorLevelOperator
	case slinkyv1beta1.AdminLevelAdministrator:
		return slurmapi.V0044UserAdministratorLevelAdministrator
	default:
		return slurmapi.V0044UserAdministratorLevelNone
	}
}

// applyLimits merges the supported AssociationLimits fields into the assoc
// without clobbering already-set fields (account/user/partition/parent).
//
// The nested v0044 association schema paths below were established empirically
// (set each limit via sacctmgr, read it back via the REST API) because the
// names are non-obvious. The slurmdbd data parser is symmetric, so writing the
// same path sets the corresponding sacctmgr limit:
//
//	MaxJobs        -> max.jobs.active
//	MaxSubmitJobs  -> max.jobs.total
//	MaxWallPerJob  -> max.jobs.per.wall_clock   (minutes)
//	MaxTRESPerJob  -> max.tres.per.job
//	GrpJobs        -> max.jobs.per.count
//	GrpSubmitJobs  -> max.jobs.per.submitted
//	GrpWall        -> max.per.account.wall_clock (minutes)
//	GrpTRES        -> max.tres.total
//
// A non-numeric Fairshare (e.g. the literal "parent") is skipped.
func applyLimits(assoc *slurmapi.V0044Assoc, l slinkyv1beta1.AssociationLimits) {
	m := map[string]any{}

	if l.Priority != nil {
		m["priority"] = noVal(*l.Priority)
	}
	if l.Fairshare != nil {
		if n, err := strconv.ParseInt(*l.Fairshare, 10, 32); err == nil {
			m["shares_raw"] = int32(n)
		}
	}
	if len(l.QOS) > 0 {
		m["qos"] = l.QOS
	}
	if l.DefaultQOS != nil {
		m["default"] = map[string]any{"qos": *l.DefaultQOS}
	}

	max := map[string]any{}
	jobs := map[string]any{}
	per := map[string]any{} // max.jobs.per
	if l.MaxJobs != nil {
		jobs["active"] = noVal(*l.MaxJobs)
	}
	if l.MaxSubmitJobs != nil {
		jobs["total"] = noVal(*l.MaxSubmitJobs)
	}
	if l.GrpJobs != nil {
		per["count"] = noVal(*l.GrpJobs)
	}
	if l.GrpSubmitJobs != nil {
		per["submitted"] = noVal(*l.GrpSubmitJobs)
	}
	if l.MaxWallPerJob != nil {
		per["wall_clock"] = noVal(int32(l.MaxWallPerJob.Minutes()))
	}
	if len(per) > 0 {
		jobs["per"] = per
	}
	if len(jobs) > 0 {
		max["jobs"] = jobs
	}
	if l.GrpWall != nil {
		max["per"] = map[string]any{
			"account": map[string]any{"wall_clock": noVal(int32(l.GrpWall.Minutes()))},
		}
	}
	tres := map[string]any{}
	if len(l.GrpTRES) > 0 {
		tres["total"] = tresList(l.GrpTRES)
	}
	if len(l.MaxTRESPerJob) > 0 {
		tres["per"] = map[string]any{"job": tresList(l.MaxTRESPerJob)}
	}
	if len(tres) > 0 {
		max["tres"] = tres
	}
	if len(max) > 0 {
		m["max"] = max
	}

	if len(m) == 0 {
		return
	}
	jsonMerge(m, assoc)
}

// tresList converts a TRES map (e.g. {"cpu":"10","gres/gpu":"4"}) into the
// slurmdbd TRES list shape. Keys may be "type" or "type/name".
func tresList(m map[string]string) []map[string]any {
	out := make([]map[string]any, 0, len(m))
	for k, v := range m {
		entry := map[string]any{}
		if i := strings.Index(k, "/"); i >= 0 {
			entry["type"] = k[:i]
			entry["name"] = k[i+1:]
		} else {
			entry["type"] = k
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			entry["count"] = n
		}
		out = append(out, entry)
	}
	return out
}

// noVal builds the slurmdbd "no-value" integer wrapper with a set value.
func noVal(n int32) map[string]any {
	return map[string]any{
		"set":    true,
		"number": n,
	}
}

// jsonMerge marshals src and unmarshals it into dst. Because json.Unmarshal
// only sets keys present in the JSON, pre-set fields on dst are preserved.
func jsonMerge(src any, dst any) {
	b, err := json.Marshal(src)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		panic(err)
	}
}

// ignoreNotModified treats slurmdbd's 304 Not Modified response as success.
// slurmdbd returns it when an upsert (POST) leaves the entity unchanged, so it
// means "already in the desired state" rather than a failure.
func ignoreNotModified(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), http.StatusText(http.StatusNotModified)) {
		return nil
	}
	return err
}

func tolerateError(err error) bool {
	if err == nil {
		return true
	}
	errText := err.Error()
	if errText == http.StatusText(http.StatusNotFound) ||
		errText == http.StatusText(http.StatusNoContent) {
		return true
	}
	return false
}
