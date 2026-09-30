package workflows_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestComposeHealthcheckExitStatus(t *testing.T) {
	raw, err := os.ReadFile("../../docker-compose.yml")
	require.NoError(t, err)
	var compose struct {
		Services map[string]struct {
			Healthcheck struct {
				Test []string `yaml:"test"`
			} `yaml:"healthcheck"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &compose))
	check := compose.Services["one-hub"].Healthcheck.Test
	require.Len(t, check, 2)
	require.Equal(t, "CMD-SHELL", check[0])
	command := strings.ReplaceAll(check[1], "$$", "$")
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "wget"), []byte("#!/bin/sh\nprintf '%s' \"$FIXTURE_BODY\"\nexit \"$FIXTURE_EXIT\"\n"), 0700))
	for _, test := range []struct {
		name, body, status string
		healthy            bool
	}{
		{"healthy", `{"success":true,"data":{}}`, "0", true},
		{"whitespace", `{ "success" : true, "data": {} }`, "0", true},
		{"unhealthy", `{"success":false}`, "0", false},
		{"connection failure", "", "1", false},
		{"HTTP failure with body", `{"success":true}`, "8", false},
		{"empty body", "", "0", false},
		{"invalid value", `{"success":trueish}`, "0", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("sh", "-c", command)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FIXTURE_BODY="+test.body, "FIXTURE_EXIT="+test.status)
			output, err := cmd.CombinedOutput()
			if test.healthy {
				require.NoError(t, err, string(output))
			} else {
				require.Error(t, err, "unhealthy service must fail the container probe")
			}
		})
	}
}
