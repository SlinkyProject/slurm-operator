// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	api "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmclientfake "github.com/SlinkyProject/slurm-client/pkg/client/fake"
	slurmerrors "github.com/SlinkyProject/slurm-client/pkg/errors"
	"github.com/SlinkyProject/slurm-client/pkg/object"

	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
)

const (
	slurmPort   = ":6820"
	webhookPort = ":8443"
	tlsCrt      = "/tls/tls.crt"
	tlsKey      = "/tls/tls.key"
)

var logger = zap.New(zap.WriteTo(io.Discard))

func main() {
	logger = zap.New()

	s := newServer(
		parseRate(os.Getenv("SLURM_ERROR_RATE")),
		parseRate(os.Getenv("KUBE_ERROR_RATE")),
		parseDuration("SLURM_DELAY"),
		parseDuration("SLURM_JITTER"),
	)
	logger.Info("mock slurmrestd",
		"slurm", s.slurmRate,
		"kube", s.kubeRate,
		"delay", s.delay,
		"jitter", s.jitter,
	)

	ctx := context.Background()
	if err := s.syncPods(ctx); err != nil {
		logger.Error(err, "pod sync")
	}
	go s.syncLoop(ctx)

	go func() {
		if err := s.serveWebhook(); err != nil {
			logger.Error(err, "webhook")
		}
	}()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.stats.mu.Lock()
			requests, failures, avg := s.stats.all.requests, s.stats.all.errors, avgMS(s.stats.all)
			s.stats.mu.Unlock()
			logger.Info("stats", "requests", requests, "errors", failures, "avg_ms", avg)
		}
	}()

	logger.Info("listening", "addr", slurmPort)
	if err := (&http.Server{
		Addr:              slurmPort,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
	}).ListenAndServe(); err != nil {
		logger.Error(err, "listen")
		os.Exit(1)
	}
}

type server struct {
	slurm     slurmclient.Client
	kube      kubeclient.Client
	mu        sync.RWMutex
	slurmRate float64
	kubeRate  float64
	delay     time.Duration
	jitter    time.Duration
	sleep     func(time.Duration)
	stats     stats
}

func newServer(slurmRate, kubeRate float64, delay, jitter time.Duration) *server {
	return &server{
		slurm:     slurmclientfake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
		slurmRate: slurmRate,
		kubeRate:  kubeRate,
		delay:     delay,
		jitter:    jitter,
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/restapi/stats" {
		s.writeStats(w)
		return
	}
	if _, _, ok := splitSlurm(r.URL.Path); !ok {
		http.NotFound(w, r)
		return
	}
	start := time.Now()
	s.wait()
	route := routeOf(r.Method, r.URL.Path)
	if fail(s.slurmRate) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": "restapi: injected slurmrestd error"}},
		})
		s.stats.record(route, true, time.Since(start))
		logger.Info("restapi", "method", r.Method, "path", r.URL.Path)
		return
	}
	code := s.dispatch(w, r)
	s.stats.record(route, code >= http.StatusInternalServerError, time.Since(start))
}

func (s *server) wait() {
	d := s.delay
	if s.jitter > 0 {
		d += time.Duration((unitFloat()*2 - 1) * float64(s.jitter))
	}
	if d <= 0 {
		return
	}
	if s.sleep != nil {
		s.sleep(d)
		return
	}
	time.Sleep(d)
}

func (s *server) dispatch(w http.ResponseWriter, r *http.Request) int {
	resource, name, ok := splitSlurm(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return http.StatusNotFound
	}
	switch resource {
	case "node", "nodes":
		return s.nodeHandler(w, r, resource, name)
	case "reservation", "reservations":
		return s.reservationHandler(w, r, resource, name)
	default:
		writeJSON(w, http.StatusOK, emptyCollections())
		return http.StatusOK
	}
}

func (s *server) nodeHandler(w http.ResponseWriter, r *http.Request, resource, name string) int {
	if resource == "nodes" || name == "" && r.Method == http.MethodGet {
		return s.listNodes(w)
	}
	switch r.Method {
	case http.MethodGet:
		return s.getNode(w, name)
	case http.MethodPost:
		return s.updateNode(w, r, name)
	case http.MethodDelete:
		return s.deleteNode(w, name)
	default:
		writeJSON(w, http.StatusOK, emptyCollections())
		return http.StatusOK
	}
}

func (s *server) listNodes(w http.ResponseWriter) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := &api.V0044NodeObjectList{}
	if err := s.slurm.List(context.Background(), list); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	body := emptyCollections()
	body["nodes"] = list.Items
	writeJSON(w, http.StatusOK, body)
	return http.StatusOK
}

func (s *server) getNode(w http.ResponseWriter, name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node := &api.V0044Node{}
	err := s.slurm.Get(context.Background(), object.ObjectKey(name), node)
	if errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, emptyCollections())
		return http.StatusNotFound
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	body := emptyCollections()
	body["nodes"] = []api.V0044Node{*node}
	writeJSON(w, http.StatusOK, body)
	return http.StatusOK
}

func (s *server) updateNode(w http.ResponseWriter, r *http.Request, name string) int {
	var msg api.V0044UpdateNodeMsg
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node := &api.V0044Node{}
	err := s.slurm.Get(context.Background(), object.ObjectKey(name), node)
	if errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusOK, emptyCollections())
		return http.StatusOK
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	if err := s.slurm.Update(context.Background(), node, msg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	writeJSON(w, http.StatusOK, emptyCollections())
	return http.StatusOK
}

func (s *server) deleteNode(w http.ResponseWriter, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	node := &api.V0044Node{Name: new(name)}
	if err := s.slurm.Delete(context.Background(), node); err != nil && !errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	writeJSON(w, http.StatusOK, emptyCollections())
	return http.StatusOK
}

func (s *server) reservationHandler(w http.ResponseWriter, r *http.Request, resource, name string) int {
	if resource == "reservations" || name == "" && r.Method == http.MethodGet {
		return s.listReservations(w)
	}
	switch r.Method {
	case http.MethodGet:
		return s.getReservation(w, name)
	case http.MethodPost:
		return s.updateReservation(w, r, name)
	case http.MethodDelete:
		return s.deleteReservation(w, name)
	default:
		writeJSON(w, http.StatusOK, emptyCollections())
		return http.StatusOK
	}
}

func (s *server) listReservations(w http.ResponseWriter) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := &api.V0044ReservationInfoObjectList{}
	if err := s.slurm.List(context.Background(), list); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	body := emptyCollections()
	body["reservations"] = list.Items
	writeJSON(w, http.StatusOK, body)
	return http.StatusOK
}

func (s *server) getReservation(w http.ResponseWriter, name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reservation := &api.V0044ReservationInfo{}
	err := s.slurm.Get(context.Background(), object.ObjectKey(name), reservation)
	if errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, emptyCollections())
		return http.StatusNotFound
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	body := emptyCollections()
	body["reservations"] = []api.V0044ReservationInfo{*reservation}
	writeJSON(w, http.StatusOK, body)
	return http.StatusOK
}

func (s *server) updateReservation(w http.ResponseWriter, r *http.Request, name string) int {
	var msg api.V0044UpdateNodeMsg
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation := &api.V0044ReservationInfo{}
	err := s.slurm.Get(context.Background(), object.ObjectKey(name), reservation)
	if errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusOK, emptyCollections())
		return http.StatusOK
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	if err := s.slurm.Update(context.Background(), reservation, msg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	writeJSON(w, http.StatusOK, emptyCollections())
	return http.StatusOK
}

func (s *server) deleteReservation(w http.ResponseWriter, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation := &api.V0044ReservationInfo{Name: new(name)}
	if err := s.slurm.Delete(context.Background(), reservation); err != nil && !errors.Is(err, slurmerrors.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"errors": []any{map[string]string{"error": err.Error()}},
		})
		return http.StatusInternalServerError
	}
	writeJSON(w, http.StatusOK, emptyCollections())
	return http.StatusOK
}

// syncNames inserts an IDLE node for each slurmd name and drops the rest.
// An empty names list deletes every node, so callers must not use it to report a failed pod list.
func (s *server) syncNames(ctx context.Context, names []string) error {
	want := make(map[string]struct{}, len(names))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range names {
		if name == "" {
			continue
		}
		want[name] = struct{}{}
		node := &api.V0044Node{}
		err := s.slurm.Get(ctx, object.ObjectKey(name), node)
		if errors.Is(err, slurmerrors.ErrNotFound) {
			if err := s.slurm.Create(ctx, idleNode(name), nil); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
	}
	list := &api.V0044NodeObjectList{}
	if err := s.slurm.List(ctx, list); err != nil {
		return err
	}
	for i := range list.Items {
		name := ptr.Deref(list.Items[i].Name, "")
		if _, ok := want[name]; ok {
			continue
		}
		if err := s.slurm.Delete(ctx, &list.Items[i]); err != nil && !errors.Is(err, slurmerrors.ErrNotFound) {
			return err
		}
	}
	return nil
}

func (s *server) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.syncPods(ctx); err != nil {
				logger.Error(err, "pod sync")
			}
		}
	}
}

func (s *server) syncPods(ctx context.Context) error {
	if s.kube == nil {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil //nolint:nilerr
		}
		scheme := runtime.NewScheme()
		if err := corev1.AddToScheme(scheme); err != nil {
			return err
		}
		s.kube, err = kubeclient.New(cfg, kubeclient.Options{Scheme: scheme})
		if err != nil {
			return err
		}
	}
	ns := namespace()
	if ns == "" {
		return nil
	}
	var names []string
	var cont string
	for {
		var pods corev1.PodList
		err := s.kube.List(ctx, &pods,
			kubeclient.InNamespace(ns),
			kubeclient.MatchingLabels{"app.kubernetes.io/name": "slurmd"},
			kubeclient.Limit(500),
			kubeclient.Continue(cont),
		)
		if err != nil {
			return err
		}
		for i := range pods.Items {
			if name := nodesetutils.GetSlurmNodeName(&pods.Items[i]); name != "" {
				names = append(names, name)
			}
		}
		cont = pods.Continue
		if cont == "" {
			break
		}
	}
	return s.syncNames(ctx, names)
}

func (s *server) serveWebhook() error {
	cert, err := tls.LoadX509KeyPair(tlsCrt, tlsKey)
	if err != nil {
		logger.Error(err, "admission webhook disabled")
		return nil
	}
	srv := &http.Server{
		Addr:              webhookPort,
		Handler:           http.HandlerFunc(s.admit),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("webhook listening", "addr", webhookPort)
	return srv.ListenAndServeTLS("", "")
}

func (s *server) admit(w http.ResponseWriter, r *http.Request) {
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := &admissionv1.AdmissionResponse{Allowed: true}
	if review.Request != nil {
		resp.UID = review.Request.UID
		if !ptr.Deref(review.Request.DryRun, false) && fail(s.kubeRate) {
			resp.Allowed = false
			resp.Result = &metav1.Status{Code: http.StatusTooManyRequests, Message: "restapi: injected Kubernetes API error"}
			logger.Info("restapi deny", "name", review.Request.Name)
		}
	}
	writeJSON(w, http.StatusOK, admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Response: resp,
	})
}

func slurmUpdateFn(_ context.Context, obj object.Object, req any, _ ...slurmclient.UpdateOption) error {
	switch o := obj.(type) {
	case *api.V0044Node:
		r, ok := req.(api.V0044UpdateNodeMsg)
		if !ok {
			return errors.New("failed to cast request object")
		}
		stateSet := set.New(ptr.Deref(o.State, []api.V0044NodeState{})...)
		statesReq := ptr.Deref(r.State, []api.V0044UpdateNodeMsgState{})
		for _, stateReq := range statesReq {
			switch stateReq {
			case api.V0044UpdateNodeMsgStateUNDRAIN:
				stateSet.Delete(api.V0044NodeStateDRAIN)
			default:
				stateSet.Insert(api.V0044NodeState(stateReq))
			}
		}
		o.State = new(stateSet.UnsortedList())
		if r.Comment != nil {
			o.Comment = r.Comment
		}
		if r.Reason != nil {
			o.Reason = r.Reason
		}
		if r.TopologyStr != nil {
			o.Topology = r.TopologyStr
		}
		if r.Features != nil {
			o.Features = r.Features
		}
		if r.FeaturesAct != nil {
			o.ActiveFeatures = r.FeaturesAct
		}
	default:
		return fmt.Errorf("unsupported slurm object: %T", o)
	}
	return nil
}

func idleNode(name string) *api.V0044Node {
	state := []api.V0044NodeState{api.V0044NodeStateIDLE}
	return &api.V0044Node{
		Name:  new(name),
		State: &state,
	}
}

func emptyCollections() map[string]any {
	return map[string]any{
		"nodes":        []api.V0044Node{},
		"jobs":         []any{},
		"partitions":   []any{},
		"reservations": []any{},
		"pings":        []any{},
		"statistics":   map[string]any{},
	}
}

func splitSlurm(path string) (resource, name string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "slurm" {
		return "", "", false
	}
	resource = parts[2]
	if len(parts) >= 4 {
		name, _ = url.PathUnescape(parts[3])
	}
	return resource, name, true
}

func routeOf(method, path string) string {
	resource, name, ok := splitSlurm(path)
	if !ok {
		return method + " other"
	}
	if resource == "node" && name != "" {
		return method + " node"
	}
	if resource == "nodes" {
		return method + " nodes"
	}
	return method + " " + resource
}

func namespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	raw, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func parseRate(raw string) float64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		logger.Info("invalid rate", "rate", raw)
		return 0
	}
	return mathutils.Clamp(value, 0, 1)
}

func parseDuration(name string) time.Duration {
	raw := os.Getenv(name)
	if raw == "" || raw == "0" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		logger.Info("invalid duration", "name", name, "value", raw)
		return 0
	}
	return d
}

func fail(chance float64) bool {
	return chance > 0 && unitFloat() < chance
}

// unitFloat returns a uniform value in [0, 1).
func unitFloat() float64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		logger.Error(err, "random")
		return 0
	}
	return float64(binary.BigEndian.Uint64(buf[:])>>11) / (1 << 53)
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(raw)
}

type counter struct {
	requests int64
	errors   int64
	latency  time.Duration
}

type stats struct {
	mu     sync.Mutex
	all    counter
	routes map[string]*counter
}

func (s *stats) record(route string, failed bool, latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.routes == nil {
		s.routes = map[string]*counter{}
	}
	add(&s.all, failed, latency)
	c := s.routes[route]
	if c == nil {
		c = &counter{}
		s.routes[route] = c
	}
	add(c, failed, latency)
}

func add(c *counter, failed bool, latency time.Duration) {
	c.requests++
	c.latency += latency
	if failed {
		c.errors++
	}
}

type routeView struct {
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	LatencyMSAvg float64 `json:"latency_ms_avg"`
}

type statsView struct {
	Requests       int64                `json:"requests"`
	Errors         int64                `json:"errors"`
	LatencyMSAvg   float64              `json:"latency_ms_avg"`
	SlurmErrorRate float64              `json:"slurm_error_rate"`
	KubeErrorRate  float64              `json:"kube_error_rate"`
	Delay          string               `json:"delay"`
	Jitter         string               `json:"jitter"`
	Routes         map[string]routeView `json:"routes"`
}

func (s *server) writeStats(w http.ResponseWriter) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()
	view := statsView{
		Requests:       s.stats.all.requests,
		Errors:         s.stats.all.errors,
		LatencyMSAvg:   avgMS(s.stats.all),
		SlurmErrorRate: s.slurmRate,
		KubeErrorRate:  s.kubeRate,
		Delay:          s.delay.String(),
		Jitter:         s.jitter.String(),
		Routes:         map[string]routeView{},
	}
	for route, c := range s.stats.routes {
		view.Routes[route] = routeView{Requests: c.requests, Errors: c.errors, LatencyMSAvg: avgMS(*c)}
	}
	writeJSON(w, http.StatusOK, view)
}

func avgMS(c counter) float64 {
	if c.requests == 0 {
		return 0
	}
	return float64(c.latency) / float64(c.requests) / float64(time.Millisecond)
}
