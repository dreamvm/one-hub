package workflows_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type workflow struct {
	On map[string]struct {
		Inputs map[string]struct {
			Default any    `yaml:"default"`
			Type    string `yaml:"type"`
		} `yaml:"inputs"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]job    `yaml:"jobs"`
}

type job struct {
	If          string            `yaml:"if"`
	Uses        string            `yaml:"uses"`
	Needs       any               `yaml:"needs"`
	With        map[string]any    `yaml:"with"`
	Env         map[string]string `yaml:"env"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []step            `yaml:"steps"`
}

type step struct {
	ID   string         `yaml:"id"`
	If   string         `yaml:"if"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

func readWorkflow(t *testing.T, name string) workflow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "workflows", name))
	require.NoError(t, err)
	var result workflow
	require.NoError(t, yaml.Unmarshal(data, &result))
	return result
}

func TestPublishingRequiresManualDispatch(t *testing.T) {
	wf := readWorkflow(t, "docker-image.yml")
	require.Len(t, wf.On, 1, "merges and tags must not publish artifacts")
	require.Contains(t, wf.On, "workflow_dispatch")
	require.Equal(t, "read", wf.Permissions["contents"])
}

func TestLegacyPublishWorkflowsAreArchived(t *testing.T) {
	for _, name := range []string{"linux-release.yml", "macos-release.yml", "windows-release.yml"} {
		require.NoFileExists(t, filepath.Join("..", "workflows", name))
		require.FileExists(t, filepath.Join("..", "legacy-workflows", name))
	}
	entries, err := os.ReadDir(filepath.Join("..", "workflows"))
	require.NoError(t, err)
	var active []string
	for _, entry := range entries {
		active = append(active, entry.Name())
	}
	require.ElementsMatch(t, []string{"docker-image.yml", "gemini-compatibility.yml", "isolated-image-smoke.yml"}, active)
}

func TestImagePublicationIsOptInAndTestGated(t *testing.T) {
	wf := readWorkflow(t, "docker-image.yml")
	publish := wf.On["workflow_dispatch"].Inputs["publish"]
	require.Equal(t, "boolean", publish.Type)
	require.Equal(t, false, publish.Default)
	require.Contains(t, wf.Jobs["resolve"].If, "github.event.repository.default_branch")
	require.Equal(t, "resolve", wf.Jobs["test"].Needs)
	require.Equal(t, "./.github/workflows/gemini-compatibility.yml", wf.Jobs["test"].Uses)
	require.Equal(t, "${{ needs.resolve.outputs.source_sha }}", wf.Jobs["test"].With["ref"])
	require.ElementsMatch(t, []any{"resolve", "test"}, wf.Jobs["image"].Needs)
	require.Empty(t, wf.Jobs["image"].If, "do not override the default successful-needs gate")
	require.Equal(t, "write", wf.Jobs["image"].Permissions["packages"])

	foundCheckout, foundLogin, foundBuild := false, false, false
	for _, item := range wf.Jobs["image"].Steps {
		switch {
		case strings.HasPrefix(item.Uses, "actions/checkout@"):
			foundCheckout = true
			require.Equal(t, "${{ needs.resolve.outputs.source_sha }}", item.With["ref"])
			require.Equal(t, false, item.With["persist-credentials"])
		case strings.HasPrefix(item.Uses, "docker/login-action@"):
			foundLogin = true
			require.Equal(t, "inputs.publish", item.If)
			require.Equal(t, "ghcr.io", item.With["registry"])
			require.Equal(t, "${{ secrets.GITHUB_TOKEN }}", item.With["password"])
		case strings.HasPrefix(item.Uses, "docker/build-push-action@"):
			foundBuild = true
			require.Equal(t, "${{ inputs.publish }}", item.With["push"])
			require.Contains(t, item.With["tags"], "${{ needs.resolve.outputs.image }}:${{ needs.resolve.outputs.version }}")
			require.NotContains(t, item.With["tags"], ":latest")
		}
	}
	require.True(t, foundCheckout && foundLogin && foundBuild)
	for _, item := range wf.Jobs["resolve"].Steps {
		if item.ID == "version" {
			require.Contains(t, item.Run, `git merge-base --is-ancestor "$source_sha" "$GITHUB_SHA"`)
			require.Contains(t, item.Run, "image=ghcr.io/%s")
		}
	}
}

func TestActiveActionsArePinned(t *testing.T) {
	pinned := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[a-f0-9]{40}$`)
	for _, name := range []string{"docker-image.yml", "gemini-compatibility.yml", "isolated-image-smoke.yml"} {
		wf := readWorkflow(t, name)
		for _, task := range wf.Jobs {
			for _, item := range task.Steps {
				if item.Uses != "" {
					require.Regexp(t, pinned, item.Uses)
				}
			}
		}
	}
}

func TestIsolatedSmokeCannotPublish(t *testing.T) {
	wf := readWorkflow(t, "isolated-image-smoke.yml")
	require.Equal(t, map[string]string{"contents": "read"}, wf.Permissions)
	require.Contains(t, wf.On, "pull_request")
	require.NotContains(t, wf.On, "pull_request_target")
	require.Equal(t, "test", wf.Jobs["smoke"].Needs)
	require.Empty(t, wf.Jobs["smoke"].If)
	require.Empty(t, wf.Jobs["smoke"].Permissions)
	require.Equal(t, "${{ github.event.pull_request.head.sha || github.sha }}", wf.Jobs["test"].With["ref"])
	require.Equal(t, wf.Jobs["test"].With["ref"], wf.Jobs["smoke"].Env["SOURCE_SHA"])
	var built, checked bool
	for _, item := range wf.Jobs["smoke"].Steps {
		require.NotContains(t, item.Uses, "login-action")
		require.NotContains(t, item.Run, "secrets.")
		if strings.HasPrefix(item.Uses, "actions/checkout@") {
			checked = true
			require.Equal(t, "${{ env.SOURCE_SHA }}", item.With["ref"])
			require.Equal(t, false, item.With["persist-credentials"])
		}
		if strings.HasPrefix(item.Uses, "docker/build-push-action@") {
			built = true
			require.Equal(t, false, item.With["push"])
			require.Equal(t, true, item.With["load"])
			require.Equal(t, "linux/amd64", item.With["platforms"])
			require.Equal(t, "onehub-isolated-smoke:${{ env.SOURCE_SHA }}", item.With["tags"])
		}
	}
	require.True(t, built && checked)
}

func TestReleaseTagValidation(t *testing.T) {
	wf := readWorkflow(t, "docker-image.yml")
	var script string
	for _, item := range wf.Jobs["resolve"].Steps {
		if item.ID == "version" {
			script = item.Run
		}
	}
	require.NotEmpty(t, script)
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		options := []string{"-c", "user.name=Workflow Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
		cmd := exec.Command("git", append(options, args...)...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return strings.TrimSpace(string(output))
	}
	git("init", "--initial-branch=main", "--quiet")
	git("commit", "--allow-empty", "-m", "validated main fixture")
	mainSHA := git("rev-parse", "HEAD")
	git("tag", "v1.0.0")
	git("tag", "-a", "v1.0.0-fixture.1", "-m", "annotated fixture")
	git("commit", "--allow-empty", "-m", "not in dispatch revision")
	git("tag", "v2.0.0")

	for _, fixture := range []struct {
		name string
		tag  string
		ok   bool
	}{
		{"lightweight", "v1.0.0", true},
		{"annotated", "v1.0.0-fixture.1", true},
		{"not_merged", "v2.0.0", false},
		{"missing", "v3.0.0", false},
		{"branch_not_tag", "main", false},
		{"shell_metacharacters", "v1.0.0; exit 0", false},
		{"command_substitution", "v1.0.0$(exit 0)", false},
		{"output_injection", "v1.0.0\nimage=unexpected", false},
		{"oversized", "v1.0.0-" + strings.Repeat("a", 128), false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			outputFile := filepath.Join(t.TempDir(), "outputs")
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "RELEASE_TAG="+fixture.tag, "GITHUB_SHA="+mainSHA, "GITHUB_REPOSITORY=DreamVM/one-hub", "GITHUB_OUTPUT="+outputFile)
			output, err := cmd.CombinedOutput()
			if !fixture.ok {
				require.Error(t, err, "%s", output)
				_, err = os.Stat(outputFile)
				require.True(t, os.IsNotExist(err))
				return
			}
			require.NoError(t, err, "%s", output)
			values, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			require.Equal(t, "source_sha="+mainSHA+"\nimage=ghcr.io/dreamvm/one-hub\nversion="+fixture.tag+"\n", string(values))
		})
	}
}
