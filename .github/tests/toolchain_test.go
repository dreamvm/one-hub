package workflows_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerUsesVerifiedGoToolchain(t *testing.T) {
	raw, err := os.ReadFile("../../Dockerfile")
	require.NoError(t, err)
	require.Contains(t, string(raw), "FROM golang:1.25.14@sha256:699337d620559a59b4a2bb298ad59611e535d2ee755a34cf2d2a98f37578dc80 AS builder2")
	require.Contains(t, string(raw), "GOTOOLCHAIN=local")
}

func TestSmokeVerifiesCandidateBinaryToolchain(t *testing.T) {
	wf := readWorkflow(t, "isolated-image-smoke.yml")
	var seenFixture, verified bool
	for _, item := range wf.Jobs["smoke"].Steps {
		if item.ID == "fixtures" {
			seenFixture = true
		}
		if item.ID == "toolchain" {
			require.True(t, seenFixture)
			require.Empty(t, item.If)
			require.Nil(t, item.ContinueOnError)
			require.Contains(t, item.Run, `"$CANDIDATE_IMAGE_ID"`)
			require.Contains(t, item.Run, "docker cp")
			require.Contains(t, item.Run, "go run ./.github/smoke/buildinfo")
			require.Contains(t, item.Run, "-go-version go1.25.14")
			verified = true
		}
		if item.ID == "sqlite" {
			require.True(t, verified, "binary identity must be checked before business smoke")
		}
	}
	require.True(t, verified)
}
