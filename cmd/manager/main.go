// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/accounting"
	"github.com/SlinkyProject/slurm-operator/internal/controller/controller"
	"github.com/SlinkyProject/slurm-operator/internal/controller/loginset"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset"
	"github.com/SlinkyProject/slurm-operator/internal/controller/restapi"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr"
	"github.com/SlinkyProject/slurm-operator/internal/controller/slurmclient"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const defaultProfileAddr = "localhost:6060"

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// Input flags to the command
type Flags struct {
	enableLeaderElection     bool
	leaderElectionNamespace  string
	probeAddr                string
	metricsAddr              string
	secureMetrics            bool
	enableHTTP2              bool
	namespaces               string
	propagatedNodeConditions string
	profile                  bool
	profileAddr              string
}

func parseFlags(flags *Flags) {
	flag.BoolVar(&flags.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(
		&flags.probeAddr,
		"health-addr",
		":8081",
		"The address the probe endpoint binds to.",
	)
	flag.BoolVar(
		&flags.enableLeaderElection,
		"leader-elect",
		false,
		("Enable leader election for controller manager. " +
			"Enabling this will ensure there is only one active controller manager."),
	)
	flag.StringVar(
		&flags.leaderElectionNamespace,
		"leader-elect-namespace",
		"",
		"Determines the namespace in which the leader election resource will be created.",
	)
	flag.StringVar(
		&flags.metricsAddr,
		"metrics-addr",
		":8080",
		"The address the metrics server binds to.",
	)
	flag.BoolVar(&flags.secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.StringVar(&flags.namespaces, "namespaces", "",
		"Comma-separated list of namespaces the controller will watch. If empty, all namespaces are watched.")
	flag.StringVar(&flags.propagatedNodeConditions, "propagated-node-conditions", "",
		"Comma-separated list of Kube node conditions, by type field, the controller will parse when setting drain reason on Slurm nodes.")
	flag.BoolVar(&flags.profile, "profile", false,
		"If set the Go profiling endpoints will be exposed via HTTP")
	flag.StringVar(
		&flags.profileAddr,
		"profile-addr",
		defaultProfileAddr,
		"The address the Go profiling endpoint binds to. This should never be exposed publicly. If empty and profiling is enabled, defaults to localhost:6060.",
	)
	flag.Parse()
}

func profileAddr(addr string) string {
	if addr == "" {
		return defaultProfileAddr
	}
	return addr
}

func newProfileMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func startProfileServer(addr string) (*http.Server, error) {
	server := &http.Server{
		Addr:         profileAddr(addr),
		Handler:      newProfileMux(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen on pprof address %q: %w", server.Addr, err)
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			setupLog.Error(err, "pprof server failed")
		}
	}()

	return server, nil
}

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;create;update;patch;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;patch

func main() {
	var flags Flags
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	parseFlags(&flags)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if flags.profile {
		if _, err := startProfileServer(flags.profileAddr); err != nil {
			setupLog.Error(err, "unable to start pprof server")
			os.Exit(1)
		}
	}

	tlsOpts := []func(*tls.Config){}
	if !flags.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	var defaultNamespaces map[string]cache.Config
	if flags.namespaces != "" {
		defaultNamespaces = make(map[string]cache.Config)
		for ns := range strings.SplitSeq(flags.namespaces, ",") {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				defaultNamespaces[ns] = cache.Config{}
			}
		}
		setupLog.Info("watching namespaces", "namespaces", flags.namespaces)
	}

	var propagatedNodeConditions []corev1.NodeConditionType
	if flags.propagatedNodeConditions != "" {
		propagatedNodeConditions = make([]corev1.NodeConditionType, 0)
		for nodeCondType := range strings.SplitSeq(flags.propagatedNodeConditions, ",") {
			nodeCondType = strings.TrimSpace(nodeCondType)
			if nodeCondType != "" {
				propagatedNodeConditions = append(propagatedNodeConditions, corev1.NodeConditionType(nodeCondType))
			}
		}
		setupLog.Info("propagated node conditions", "propagatedNodeConditions", flags.propagatedNodeConditions)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			TLSOpts:     tlsOpts,
			BindAddress: flags.metricsAddr,
		},
		Cache: cache.Options{
			DefaultNamespaces: defaultNamespaces,
		},
		HealthProbeBindAddress:        flags.probeAddr,
		LeaderElection:                flags.enableLeaderElection,
		LeaderElectionID:              "slurm-operator",
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionNamespace:       flags.leaderElectionNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clientMap := clientmap.NewClientMap()
	if err := controller.NewReconciler(mgr.GetClient(), clientMap).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Controller")
		os.Exit(1)
	}
	if err := restapi.NewReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Restapi")
		os.Exit(1)
	}
	if err := accounting.NewReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Accounting")
		os.Exit(1)
	}
	if err := nodeset.NewReconciler(mgr.GetClient(), clientMap, propagatedNodeConditions).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeSet")
		os.Exit(1)
	}
	if err := loginset.NewReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LoginSet")
		os.Exit(1)
	}
	if err := slurmclient.NewReconciler(mgr.GetClient(), clientMap).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SlurmClient")
		os.Exit(1)
	}
	if err := token.NewReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Token")
		os.Exit(1)
	}
	if err := sacctmgr.NewAccountReconciler(mgr.GetClient(), clientMap).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Account")
		os.Exit(1)
	}
	if err := sacctmgr.NewUserReconciler(mgr.GetClient(), clientMap).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "User")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running controller")
		os.Exit(1)
	}
}
