// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import "sigs.k8s.io/e2e-framework/pkg/env"

type SlurmImageConfig struct {
	Tag  string
	Repo string
}

type SlurmInstallationConfig struct {
	Accounting bool
	Login      bool
	Metrics    bool
	Namespace  string
	Pyxis      bool
}

var (
	Testenv         env.Environment
	TestUID         string
	SlinkyNamespace string = "slinky"
	Basepath        string
	SlurmImage      SlurmImageConfig
)
