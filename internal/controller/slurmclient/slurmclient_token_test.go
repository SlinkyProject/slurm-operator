// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	client "github.com/SlinkyProject/slurm-client/pkg/client"
	clienttoken "github.com/SlinkyProject/slurm-client/pkg/client/token"
	"github.com/SlinkyProject/slurm-client/pkg/types"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

type trackingSlurmClient struct {
	client.Client

	server                string
	token                 string
	setServerCalls        int
	setTokenProviderCalls int
}

func (c *trackingSlurmClient) GetServer() string {
	return c.server
}

func (c *trackingSlurmClient) SetServer(server string) {
	c.server = server
	c.setServerCalls++
}

func (c *trackingSlurmClient) SetTokenProvider(provider clienttoken.Provider) {
	c.token, _ = provider.Token(context.Background())
	c.setTokenProviderCalls++
}

func TestUpdateClient(t *testing.T) {
	t.Run("token only", func(t *testing.T) {
		slurmClient := &trackingSlurmClient{server: "http://slurmrestd:6820"}

		updateClient(slurmClient, "http://slurmrestd:6820", "rotated-token")

		if slurmClient.setServerCalls != 0 {
			t.Fatalf("SetServer() calls = %d, want 0", slurmClient.setServerCalls)
		}
		if slurmClient.setTokenProviderCalls != 1 {
			t.Fatalf("SetTokenProvider() calls = %d, want 1", slurmClient.setTokenProviderCalls)
		}
		if slurmClient.token != "rotated-token" {
			t.Fatalf("token = %q, want %q", slurmClient.token, "rotated-token")
		}
	})

	t.Run("server and token", func(t *testing.T) {
		slurmClient := &trackingSlurmClient{server: "http://old-slurmrestd:6820"}

		updateClient(slurmClient, "http://new-slurmrestd:6820", "rotated-token")

		if slurmClient.setServerCalls != 1 {
			t.Fatalf("SetServer() calls = %d, want 1", slurmClient.setServerCalls)
		}
		if slurmClient.server != "http://new-slurmrestd:6820" {
			t.Fatalf("server = %q, want %q", slurmClient.server, "http://new-slurmrestd:6820")
		}
		if slurmClient.setTokenProviderCalls != 1 {
			t.Fatalf("SetTokenProvider() calls = %d, want 1", slurmClient.setTokenProviderCalls)
		}
	})
}

func TestSetTokenProviderUpdatesNextRequest(t *testing.T) {
	const (
		initialToken = "initial-token"
		rotatedToken = "rotated-token"
	)

	requestTokens := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestTokens <- req.Header.Get("X-SLURM-USER-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pings":[]}`))
	}))
	t.Cleanup(server.Close)

	slurmClient, err := client.NewClient(&client.Config{
		Server:        server.URL,
		TokenProvider: clienttoken.StaticProvider(initialToken),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	listPing := func() {
		t.Helper()
		list := &types.V0045ControllerPingList{}
		if err := slurmClient.List(context.Background(), list, &client.ListOptions{SkipCache: true}); err != nil {
			t.Fatalf("List() error = %v", err)
		}
	}

	listPing()
	slurmClient.SetTokenProvider(clienttoken.StaticProvider(rotatedToken))
	listPing()

	if got := <-requestTokens; got != initialToken {
		t.Fatalf("initial request token = %q, want %q", got, initialToken)
	}
	if got := <-requestTokens; got != rotatedToken {
		t.Fatalf("rotated request token = %q, want %q", got, rotatedToken)
	}
}

func TestMapSecretToControllers(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, slinkyv1beta1.AddToScheme(scheme))

	jwtKeyRef := testutils.NewJwtKeyRef("slurm")
	controller := testutils.NewController(
		"slurm",
		testutils.NewSlurmKeyRef("slurm"),
		jwtKeyRef,
		nil,
	)
	otherController := testutils.NewController(
		"other",
		testutils.NewSlurmKeyRef("other"),
		testutils.NewJwtKeyRef("other"),
		nil,
	)
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(controller, otherController).
		Build()
	reconciler := &SlurmClientReconciler{Client: kubeClient}

	requests := reconciler.mapSecretToControllers(
		context.Background(),
		testutils.NewJwtKeySecret(jwtKeyRef),
	)

	require.Equal(t, []reconcile.Request{{
		NamespacedName: k8sclient.ObjectKeyFromObject(controller),
	}}, requests)

	requests = reconciler.mapSecretToControllers(
		context.Background(),
		testutils.NewJwtKeySecret(testutils.NewJwtKeyRef("unreferenced")),
	)
	require.Empty(t, requests)
}
