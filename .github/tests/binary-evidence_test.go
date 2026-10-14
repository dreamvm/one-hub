package workflows_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinaryEvidenceIsOptInAndCannotReplaceAcceptance(t *testing.T) {
	wf := readWorkflow(t, "isolated-image-smoke.yml")
	input := wf.On["workflow_dispatch"].Inputs["export_binary"]
	require.Equal(t, "boolean", input.Type)
	require.Equal(t, false, input.Default)
	require.Equal(t, map[string]string{"contents": "read"}, wf.Permissions)
	const gate = "success() && (github.event_name == 'workflow_dispatch' && inputs.export_binary || github.event_name == 'pull_request' && contains(github.event.pull_request.labels.*.name, 'binary-audit-export'))"
	var sawCompose, prepared, uploaded bool
	for _, item := range wf.Jobs["smoke"].Steps {
		if strings.Contains(item.Run, "python3 .github/smoke/compose.py") {
			sawCompose = true
		}
		if item.ID == "binary-evidence" {
			require.True(t, sawCompose, "export must follow the last acceptance path")
			require.Equal(t, gate, item.If)
			require.Nil(t, item.ContinueOnError)
			require.Contains(t, item.Run, `[[ "$(git rev-parse HEAD)" == "$SOURCE_SHA" ]]`)
			require.Contains(t, item.Run, "set -euo pipefail")
			require.Contains(t, item.Run, "python3 .github/smoke/export_binary.py")
			prepared = true
		}
		if item.ID == "binary-upload" {
			require.True(t, prepared)
			require.Equal(t, gate, item.If)
			require.Nil(t, item.ContinueOnError)
			require.Equal(t, "actions/upload-artifact@bbbca2ddaa5d8feaa63e36b76fdaad77386f024f", item.Uses)
			require.Equal(t, "onehub-candidate-${{ env.SOURCE_SHA }}-${{ matrix.arch }}-${{ github.run_attempt }}", item.With["name"])
			require.Equal(t, "${{ runner.temp }}/candidate-evidence/one-api\n${{ runner.temp }}/candidate-evidence/manifest.json\n", item.With["path"])
			require.Equal(t, 7, item.With["retention-days"])
			require.Equal(t, "error", item.With["if-no-files-found"])
			require.Equal(t, false, item.With["overwrite"])
			require.Equal(t, false, item.With["include-hidden-files"])
			uploaded = true
		}
	}
	require.True(t, prepared && uploaded)
}
