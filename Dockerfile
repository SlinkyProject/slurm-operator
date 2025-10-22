# syntax=docker/dockerfile:1
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

################################################################################
FROM --platform=${BUILDPLATFORM} golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/bin/ ./...

################################################################################

# Ref: https://github.com/GoogleContainerTools/distroless
FROM gcr.io/distroless/static:nonroot AS manager
WORKDIR /
COPY --from=builder /workspace/bin/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]

################################################################################

# Ref: https://github.com/GoogleContainerTools/distroless
FROM gcr.io/distroless/static:nonroot AS webhook
WORKDIR /
COPY --from=builder /workspace/bin/webhook .
USER 65532:65532
ENTRYPOINT ["/webhook"]
