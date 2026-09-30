package workflows_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEntryPreservesInputsAndPropagatesFailure(t *testing.T) {
	task := os.Getenv("ONEHUB_TASK_BIN")
	if task == "" {
		var err error
		task, err = exec.LookPath("task")
		if err != nil {
			t.Skip("fixed Task runtime not provided")
		}
	}
	task, err := filepath.Abs(task)
	require.NoError(t, err)
	for _, stage := range []string{"normal", "install failure", "frontend failure", "backend failure", "docker local"} {
		t.Run(stage, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "fixture with spaces")
			for _, d := range []string{"web/build", "web/src", "hack/scripts", "tools"} {
				require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0700))
			}
			for _, name := range []string{"Taskfile.yml", "hack/scripts/genui.sh"} {
				data, err := os.ReadFile(filepath.Join("../..", name))
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(root, name), data, 0700))
			}
			inputs := map[string]string{"go.mod": "module fixture\n", "go.sum": "fixture sum\n", "web/package.json": "{\"name\":\"fixture\",\"version\":\"1.0.0\"}\n", "web/yarn.lock": "fixture lock\n"}
			for name, data := range inputs {
				require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(data), 0600))
			}
			require.NoError(t, os.WriteFile(filepath.Join(root, "web/build/index.html"), []byte("stale output"), 0600))
			for name, body := range map[string]string{
				"git":    "#!/bin/sh\ncase \"$1\" in describe) echo v0.0.0-fixture;; show) echo 20260930;; rev-parse) echo fixturecommit;; esac\n",
				"go":     "#!/bin/sh\nprintf 'go %s\\n' \"$*\" >> \"$FIXTURE_LOG\"\ncase \"$1 $2\" in 'mod tidy') echo mutated >> go.mod;; 'build '*) [ \"$FIXTURE_STAGE\" != 'backend failure' ] || exit 9;; 'env GOOS') echo linux;; 'env GOARCH') echo amd64;; 'version ') echo 'go version go1.25.14 linux/amd64';; esac\n",
				"yarn":   "#!/bin/sh\nprintf 'yarn %s version=%s\\n' \"$*\" \"$VITE_APP_VERSION\" >> \"$FIXTURE_LOG\"\ncase \"$1\" in install) [ \"$FIXTURE_STAGE\" != 'install failure' ] || exit 7;; build) [ \"$FIXTURE_STAGE\" != 'frontend failure' ] || exit 8; printf fresh > build/index.html;; esac\n",
				"npm":    "#!/bin/sh\necho npm-unexpected >> \"$FIXTURE_LOG\"\nexit 1\n",
				"jq":     "#!/bin/sh\nprintf '{}'\n",
				"docker": "#!/bin/sh\nprintf 'docker %s\\n' \"$*\" >> \"$FIXTURE_LOG\"\n",
			} {
				require.NoError(t, os.WriteFile(filepath.Join(root, "tools", name), []byte(body), 0700))
			}
			target := "build"
			if stage == "docker local" {
				target = "docker"
			}
			log := filepath.Join(root, "calls.log")
			cmd := exec.Command(task, "--dir", root, target)
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "tools")+":"+os.Getenv("PATH"), "FIXTURE_LOG="+log, "FIXTURE_STAGE="+stage)
			output, runErr := cmd.CombinedOutput()
			if stage == "normal" || stage == "docker local" {
				require.NoError(t, runErr, string(output))
			} else {
				require.Error(t, runErr, "failed build stage must abort")
			}
			for name, want := range inputs {
				data, err := os.ReadFile(filepath.Join(root, name))
				require.NoError(t, err)
				require.Equal(t, want, string(data), name)
			}
			data, err := os.ReadFile(log)
			require.NoError(t, err)
			calls := string(data)
			require.NotContains(t, calls, "mod tidy")
			require.NotContains(t, calls, "npm-unexpected")
			require.NotContains(t, calls, "--push")
			if stage == "docker local" {
				require.Contains(t, calls, "docker build")
				return
			}
			require.Contains(t, calls, "yarn install --frozen-lockfile --non-interactive")
			if stage == "install failure" {
				require.NotContains(t, calls, "yarn build")
				require.NotContains(t, calls, "go build")
				return
			}
			require.Contains(t, calls, "yarn build version=v0.0.0-fixture")
			if stage == "frontend failure" {
				require.NotContains(t, calls, "go build")
				return
			}
			require.Contains(t, calls, "-mod=readonly")
			require.Contains(t, calls, "-trimpath")
			if stage == "normal" {
				built, err := os.ReadFile(filepath.Join(root, "web/build/index.html"))
				require.NoError(t, err)
				require.Equal(t, "fresh", strings.TrimSpace(string(built)))
			}
		})
	}
}
