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
	raw, err := os.ReadFile("../../docker-compose.sqlite.yml")
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

func TestComposeDependencyHealthchecks(t *testing.T) {
	for _, dependency := range []struct{ file, service, binary, response string }{
		{"docker-compose.mysql.yml", "db", "mysql", "1"},
		{"docker-compose.redis.yml", "redis", "redis-cli", "PONG"},
	} {
		raw, err := os.ReadFile(filepath.Join("../..", dependency.file))
		require.NoError(t, err)
		var config struct {
			Services map[string]struct {
				Healthcheck struct {
					Test []string `yaml:"test"`
				} `yaml:"healthcheck"`
			} `yaml:"services"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &config))
		probe := config.Services[dependency.service].Healthcheck.Test
		require.Len(t, probe, 2)
		require.Equal(t, "CMD-SHELL", probe[0])
		bin := t.TempDir()
		body := "#!/bin/sh\nprintf '%s' \"$FIXTURE_BODY\"\nexit \"$FIXTURE_EXIT\"\n"
		if dependency.service == "db" {
			body = "#!/bin/sh\n[ \"$MYSQL_PWD\" = \"$MYSQL_PASSWORD\" ] || exit 3\n[ \"$*\" = '--protocol=TCP -h 127.0.0.1 -u oneapi -D one-api -Nse SELECT 1' ] || exit 4\nprintf '%s' \"$FIXTURE_BODY\"\nexit \"$FIXTURE_EXIT\"\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(bin, dependency.binary), []byte(body), 0700))
		for _, fixture := range []struct {
			name, response, status string
			healthy                bool
		}{
			{"healthy", dependency.response, "0", true},
			{"connection failure", "", "1", false},
			{"auth failure", "denied", "1", false},
			{"error response", "ERR unexpected", "0", false},
			{"failed command with normal output", dependency.response, "1", false},
		} {
			t.Run(dependency.service+"/"+fixture.name, func(t *testing.T) {
				cmd := exec.Command("sh", "-c", strings.ReplaceAll(probe[1], "$$", "$"))
				cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FIXTURE_BODY="+fixture.response, "FIXTURE_EXIT="+fixture.status, "MYSQL_USER=oneapi", "MYSQL_DATABASE=one-api", "MYSQL_PASSWORD=fixture ' $; password")
				out, err := cmd.CombinedOutput()
				if fixture.healthy {
					require.NoError(t, err, string(out))
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}
