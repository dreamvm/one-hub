package workflows_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerUsesVerifiedGoToolchain(t *testing.T) {
	raw, err := os.ReadFile("../../Dockerfile")
	require.NoError(t, err)
	require.Contains(t, string(raw), "FROM golang:1.26.9@sha256:37cfcca63222ec499bec5e8ad7aad52dbf7529db36d5812b8eae2f0621aa92d6 AS builder2")
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
			require.Contains(t, item.Run, "-go-version go1.26.9")
			verified = true
		}
		if item.ID == "sqlite" {
			require.True(t, verified, "binary identity must be checked before business smoke")
		}
	}
	require.True(t, verified)
}
