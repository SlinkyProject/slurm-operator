// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"
)

func TestMockSlurmrestd(t *testing.T) {
	s := newServer(0, 0, 0, 0)
	if err := s.syncNames(t.Context(), []string{"slurm-kwok-0"}); err != nil {
		t.Fatal(err)
	}

	res := get(t, s, "/slurm/v0.0.44/nodes/")
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"name":"slurm-kwok-0"`)) || !bytes.Contains(res.Body.Bytes(), []byte(`"IDLE"`)) {
		t.Fatalf("list: %d %s", res.Code, res.Body.Bytes())
	}

	res = get(t, s, "/slurm/v0.0.44/node/missing")
	if res.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", res.Code)
	}

	body := bytes.NewBufferString(`{"comment":"pod"}`)
	req := httptest.NewRequest(http.MethodPost, "/slurm/v0.0.44/node/slurm-kwok-0", body)
	res = httptest.NewRecorder()
	s.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("update: %d %s", res.Code, res.Body.Bytes())
	}
	res = get(t, s, "/slurm/v0.0.44/node/slurm-kwok-0")
	if !bytes.Contains(res.Body.Bytes(), []byte(`"comment":"pod"`)) {
		t.Fatalf("comment not stored: %s", res.Body.Bytes())
	}

	res = get(t, s, "/slurm/v0.0.44/jobs/")
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"jobs":[]`)) {
		t.Fatalf("jobs: %d %s", res.Code, res.Body.Bytes())
	}

	if err := s.syncNames(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	res = get(t, s, "/slurm/v0.0.44/nodes/")
	if bytes.Contains(res.Body.Bytes(), []byte(`"slurm-kwok-0"`)) {
		t.Fatalf("node survived removal: %s", res.Body.Bytes())
	}
}

func TestRestapi(t *testing.T) {
	s := newServer(1, 1, time.Second, 100*time.Millisecond)
	var waits int
	s.sleep = func(d time.Duration) {
		waits++
		if d < 900*time.Millisecond || d > 1100*time.Millisecond {
			t.Errorf("delay outside jitter range: %s", d)
		}
	}

	res := get(t, s, "/slurm/v0.0.44/nodes/")
	if res.Code != http.StatusInternalServerError || waits != 1 {
		t.Fatalf("restapi: code=%d waits=%d", res.Code, waits)
	}
	res = get(t, s, "/restapi/stats")
	if res.Code != http.StatusOK || waits != 1 {
		t.Fatalf("stats should skip delay: %d waits=%d", res.Code, waits)
	}
	var view statsView
	if err := json.Unmarshal(res.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Requests != 1 || view.Errors != 1 || view.SlurmErrorRate != 1 || view.Delay != "1s" || view.Jitter != "100ms" {
		t.Fatalf("stats: %+v", view)
	}
	res = get(t, s, "/choas/stats")
	if res.Code != http.StatusNotFound || waits != 1 {
		t.Fatalf("misspelled stats path: code=%d waits=%d body=%s", res.Code, waits, res.Body.Bytes())
	}

	s.delay = 0
	s.jitter = 0
	review := admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: "abc", Name: "slurm-kwok-0"}}
	raw, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/admit", bytes.NewReader(raw))
	res = httptest.NewRecorder()
	s.admit(res, req)
	if err := json.Unmarshal(res.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	if review.Response == nil || review.Response.Allowed {
		t.Fatalf("expected deny: %s", res.Body.Bytes())
	}

	review = admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: "abc", DryRun: ptr.To(true)}}
	raw, _ = json.Marshal(review)
	req = httptest.NewRequest(http.MethodPost, "/admit", bytes.NewReader(raw))
	res = httptest.NewRecorder()
	s.admit(res, req)
	if err := json.Unmarshal(res.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	if review.Response == nil || !review.Response.Allowed {
		t.Fatalf("dryRun should allow: %s", res.Body.Bytes())
	}
}

func TestParseRateAndDuration(t *testing.T) {
	if parseRate("0.2") != 0.2 || parseRate("") != 0 || parseRate("nope") != 0 {
		t.Fatalf("rate: %v %v", parseRate("0.2"), 0.2)
	}
	t.Setenv("TEST_DURATION", "20ms")
	if parseDuration("TEST_DURATION") != 20*time.Millisecond {
		t.Fatalf("duration: %s", parseDuration("TEST_DURATION"))
	}
	t.Setenv("TEST_DURATION", "nope")
	if parseDuration("TEST_DURATION") != 0 {
		t.Fatalf("invalid duration was accepted")
	}
}

func get(t *testing.T, s *server, path string) *httptest.ResponseRecorder {
	t.Helper()
	res := httptest.NewRecorder()
	s.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
	return res
}
