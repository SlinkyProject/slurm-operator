// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	dockerbuild "github.com/docker/docker/api/types/build"
	dockerclient "github.com/docker/docker/client"
	ptr "k8s.io/utils/ptr"

	"github.com/moby/go-archive"
)

// getBasePath returns the fully qualified path of the slurm-operator repo within the context in which `go test` is called
func GetBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	fullpath := filepath.Dir(b)
	path, _ := strings.CutSuffix(fullpath, "test")

	return path
}

// BuildOperatorImages builds images for Slurm-operator and Slurm-operator-webhook
func BuildOperatorImages(operatorName string, webhookName string) error {
	imageOS := runtime.GOOS
	imageArch := runtime.GOARCH

	imagePlatform := imageOS + "/" + imageArch
	buildArgs := map[string]*string{
		"TARGETOS":      ptr.To(imageOS),
		"TARGETARCH":    ptr.To(imageArch),
		"BUILDPLATFORM": ptr.To(imagePlatform),
	}

	// Build slurm-operator image
	var operatorTags []string
	operatorTags = append(operatorTags, operatorName)
	err := DockerBuild(operatorTags, "manager", "Dockerfile", Basepath, buildArgs)
	if err != nil {
		return err
	}

	// Build slurm-operator-webhook image
	var webhookTags []string
	webhookTags = append(webhookTags, webhookName)
	err = DockerBuild(webhookTags, "webhook", "Dockerfile", Basepath, buildArgs)
	if err != nil {
		return err
	}

	return nil
}

// DockerBuild builds a Docker image from the provided parameters and pushes it to the local registry
func DockerBuild(imageTags []string, imageTarget string, dockerfile string, dockerfilePath string, buildArgs map[string]*string) error {
	ctx := context.Background()
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err, " :unable to init client")
	}

	tar, err := archive.TarWithOptions(dockerfilePath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := dockerbuild.ImageBuildOptions{
		Context:    tar,
		Dockerfile: dockerfile,
		Remove:     true,
		Target:     imageTarget,
		Tags:       imageTags,
		BuildArgs:  buildArgs,
	}

	imageBuildResponse, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		return err
	}

	return nil
}

func RetryCommand(ctx context.Context, t *testing.T, command string, args []string, wants string, cleanup_command string, cleanup_args []string, retries int, retryDelay time.Duration) context.Context {
	for retry := range retries {

		if cleanup_command != "" && len(cleanup_args) > 0 {
			cleanup_cmd := exec.Command(cleanup_command, cleanup_args...)

			_, _ = cleanup_cmd.Output() //nolint:errcheck
		}

		cmd := exec.Command(command, args...)

		output, err := cmd.Output()
		if err == nil && (wants == "" || strings.TrimSpace(string(output)) == wants) {
			return ctx
		}

		if retry == retries-retry {
			if err != nil {
				t.Fatalf("failed running '%v %v': %v", command, args, err)
			}
			if string(output) != "" {
				t.Fatalf("assertion failed. wants: %v, got: %v", wants, string(output))
			}

			return ctx
		}

		time.Sleep(retryDelay)
	}

	return ctx
}

func GetSlurmNodeInfo(nodeName string) (map[string]string, error) {
	command := "kubectl"
	args := []string{
		"exec", "-n", SlurmNamespace, "slurm-controller-0", "--",
		"scontrol", "show", "node", nodeName,
	}

	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("failed executing command")
	}

	out_map := StringToMap(string(output))
	return out_map, nil
}

func StringToMap(input string) map[string]string {
	out_array := strings.Split(string(input), " ")
	out_map := make(map[string]string)

	for _, val := range out_array {
		object := strings.Split(val, "=")
		if len(object) == 2 {
			key := object[0]
			value := object[1]

			out_map[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	return out_map
}
