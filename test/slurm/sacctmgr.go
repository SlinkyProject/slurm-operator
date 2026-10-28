// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// PingAccounting checks that slurmdbd responds.
func (c *Client) PingAccounting(ctx context.Context) error {
	_, err := c.sacctmgr(ctx, "ping")
	return err
}

// Clusters returns the names of the clusters registered in slurmdbd.
func (c *Client) Clusters(ctx context.Context) ([]string, error) {
	return c.sacctmgrShow(ctx, "cluster", "Cluster")
}

// AddAccount creates an account.
func (c *Client) AddAccount(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("account name must not be empty")
	}
	_, err := c.sacctmgr(ctx, "--immediate", "add", "account", "name="+name)
	return err
}

// HasAccount reports whether the account exists.
func (c *Client) HasAccount(ctx context.Context, name string) (bool, error) {
	accounts, err := c.sacctmgrShow(ctx, "account", "Account", "name="+name)
	return slices.Contains(accounts, name), err
}

// DeleteAccount removes an account and its associations.
func (c *Client) DeleteAccount(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("account name must not be empty")
	}
	_, err := c.sacctmgr(ctx, "--immediate", "delete", "account", "name="+name)
	return err
}

// AddUser creates a user, or adds an association for an existing one, with
// account as one of its accounts.
func (c *Client) AddUser(ctx context.Context, name, account string) error {
	if name == "" || account == "" {
		return errors.New("user and account names must not be empty")
	}
	_, err := c.sacctmgr(ctx, "--immediate", "add", "user", "name="+name, "account="+account)
	return err
}

// HasUser reports whether the user exists.
func (c *Client) HasUser(ctx context.Context, name string) (bool, error) {
	users, err := c.sacctmgrShow(ctx, "user", "User", "name="+name)
	return slices.Contains(users, name), err
}

// HasAssociation reports whether the user is associated with the account.
func (c *Client) HasAssociation(ctx context.Context, user, account string) (bool, error) {
	accounts, err := c.sacctmgrShow(ctx, "association", "Account", "user="+user, "account="+account)
	return slices.Contains(accounts, account), err
}

func (c *Client) sacctmgr(ctx context.Context, args ...string) (string, error) {
	return c.Exec(ctx, append([]string{"sacctmgr"}, args...)...)
}

// sacctmgrShow lists one field of the entities matching filters.
func (c *Client) sacctmgrShow(ctx context.Context, entity, field string, filters ...string) ([]string, error) {
	args := append([]string{"--noheader", "--parsable2", "show", entity}, filters...)
	stdout, err := c.sacctmgr(ctx, append(args, "format="+field)...)
	if err != nil {
		return nil, err
	}
	return parseColumn(stdout), nil
}

// parseColumn returns the first "|"-separated value of each non-empty line.
func parseColumn(output string) []string {
	var values []string
	for line := range strings.Lines(output) {
		value, _, _ := strings.Cut(strings.TrimSpace(line), "|")
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
