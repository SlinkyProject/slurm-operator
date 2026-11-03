// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestE2EConfigSelectsContext(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	selected := filepath.Join(dir, "selected")
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		CurrentContext: "other",
		Contexts: map[string]*clientcmdapi.Context{
			"other": {Cluster: "other"},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"other": {Server: "https://other.invalid"},
		},
	}, current))
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		Contexts: map[string]*clientcmdapi.Context{
			"e2e-selected": {Cluster: "selected"},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"selected": {Server: "https://selected.invalid"},
		},
	}, selected))
	t.Setenv("KUBECONFIG", current+string(os.PathListSeparator)+selected)
	t.Setenv(E2EKubeContextEnvironment, "e2e-selected")

	path := filepath.Join(dir, "scoped")
	cfg, err := E2EConfig(path)
	require.NoError(t, err)
	require.Equal(t, "e2e-selected", cfg.KubeContext())
	client, err := cfg.NewClient()
	require.NoError(t, err)
	require.Equal(t, "https://selected.invalid", client.RESTConfig().Host)

	// Subprocesses read the same file and cannot fall back to the other cluster.
	scoped, err := clientcmd.LoadFromFile(cfg.KubeconfigFile())
	require.NoError(t, err)
	require.Equal(t, "e2e-selected", scoped.CurrentContext)
	require.Len(t, scoped.Contexts, 1)
	require.Len(t, scoped.Clusters, 1)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	original, err := clientcmd.LoadFromFile(current)
	require.NoError(t, err)
	require.Equal(t, "other", original.CurrentContext)
}

func TestE2EConfigRejectsMissingContext(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		CurrentContext: "other",
		Contexts:       map[string]*clientcmdapi.Context{"other": {Cluster: "other"}},
		Clusters:       map[string]*clientcmdapi.Cluster{"other": {Server: "https://other.invalid"}},
	}, source))
	t.Setenv("KUBECONFIG", source)
	for _, context := range []string{"", "missing"} {
		t.Run(context, func(t *testing.T) {
			t.Setenv(E2EKubeContextEnvironment, context)
			path := filepath.Join(t.TempDir(), "scoped")
			_, err := E2EConfig(path)
			require.Error(t, err)
			require.NoFileExists(t, path)
		})
	}
}

func TestE2EConfigPreservesRelativeCredentials(t *testing.T) {
	dir := t.TempDir()
	ca := []byte("test certificate data")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600))
	source := filepath.Join(dir, "source")
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		Contexts: map[string]*clientcmdapi.Context{"selected": {Cluster: "selected"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"selected": {Server: "https://selected.invalid", CertificateAuthority: "ca.crt"},
		},
	}, source))
	t.Setenv("KUBECONFIG", source)
	t.Setenv(E2EKubeContextEnvironment, "selected")
	cfg, err := E2EConfig(filepath.Join(t.TempDir(), "scoped"))
	require.NoError(t, err)
	scoped, err := clientcmd.LoadFromFile(cfg.KubeconfigFile())
	require.NoError(t, err)
	require.Empty(t, scoped.Clusters["selected"].CertificateAuthority)
	require.Equal(t, ca, scoped.Clusters["selected"].CertificateAuthorityData)
}
