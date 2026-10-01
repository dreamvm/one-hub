package workflows_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSmokeRunsNativeArchitecturesWithMatchingToolsAndImages(t *testing.T) {
	smoke := readWorkflow(t, "isolated-image-smoke.yml").Jobs["smoke"]
	require.NotNil(t, smoke.Strategy.FailFast, "one architecture failure must not cancel evidence for the other")
	require.False(t, *smoke.Strategy.FailFast)
	require.Equal(t, "${{ matrix.runner }}", smoke.RunsOn)
	architectures := map[string]string{}
	for _, target := range smoke.Strategy.Matrix.Include {
		require.NotContains(t, architectures, target.Arch)
		architectures[target.Arch] = target.Runner
	}
	require.Equal(t, map[string]string{"amd64": "ubuntu-24.04", "arm64": "ubuntu-24.04-arm"}, architectures)
	require.Equal(t, "${{ matrix.arch }}", smoke.Env["TARGET_ARCH"])
	var mock, build, pull, binary bool
	for _, item := range smoke.Steps {
		if strings.Contains(item.Run, "go build") && strings.Contains(item.Run, "./.github/smoke/mockserver/cmd") {
			mock = true
			require.Equal(t, "${{ matrix.arch }}", item.Env["GOARCH"])
		}
		if item.With["platforms"] != nil {
			build = true
			require.Equal(t, "linux/${{ matrix.arch }}", item.With["platforms"])
		}
		if item.ID == "fixtures" {
			pull = true
			require.Contains(t, item.Run, `docker pull --platform "linux/$TARGET_ARCH" "$reference"`)
			require.Contains(t, item.Run, `{{.Architecture}}`)
			require.Contains(t, item.Run, `"$TARGET_ARCH"`)
		}
		if item.ID == "toolchain" {
			binary = true
			require.Contains(t, item.Run, `-goarch "$TARGET_ARCH"`)
		}
	}
	require.True(t, mock && build && pull && binary)
}
