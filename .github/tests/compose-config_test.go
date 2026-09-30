package workflows_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var composeFixture = map[string]string{
	"ONEHUB_IMAGE_DIGEST":        "sha256:" + strings.Repeat("a", 64),
	"ONEHUB_SESSION_SECRET":      strings.Repeat("b", 64),
	"ONEHUB_USER_TOKEN_SECRET":   strings.Repeat("c", 64),
	"ONEHUB_MYSQL_PASSWORD":      strings.Repeat("d", 64),
	"ONEHUB_MYSQL_ROOT_PASSWORD": strings.Repeat("e", 64),
}

func composeConfig(t *testing.T, files []string, missing string) ([]byte, error) {
	t.Helper()
	bin := os.Getenv("ONEHUB_COMPOSE_BIN")
	if bin == "" {
		t.Skip("fixed Compose runtime not provided")
	}
	// Copy only templates: never consume developer .env files or Docker credentials.
	dir := t.TempDir()
	names, err := filepath.Glob("../../docker-compose*.yml")
	require.NoError(t, err)
	for _, name := range names {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.Base(name)), data, 0600))
	}
	args := []string{"--project-name", "onehub-fixture", "--env-file", os.DevNull}
	for _, file := range files {
		args = append(args, "-f", filepath.Join(dir, file))
	}
	args = append(args, "config", "--format", "json")
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "DOCKER_CONFIG=" + dir}
	for key, value := range composeFixture {
		if !strings.Contains(","+missing+",", ","+key+",") {
			if missing == "empty:"+key {
				value = ""
			}
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	return cmd.Output()
}

func TestComposeDefaultRequiresExplicitConfiguration(t *testing.T) {
	t.Run("configured control", func(t *testing.T) {
		_, err := composeConfig(t, []string{"docker-compose.yml"}, "")
		require.NoError(t, err)
	})
	for key := range composeFixture {
		for _, prefix := range []string{"", "empty:"} {
			t.Run(prefix+"missing "+key, func(t *testing.T) {
				_, err := composeConfig(t, []string{"docker-compose.yml"}, prefix+key)
				require.Error(t, err, "missing explicit configuration must fail closed")
			})
		}
	}
}

func TestComposeDefaultBoundary(t *testing.T) {
	data, err := composeConfig(t, []string{"docker-compose.yml"}, "")
	require.NoError(t, err)
	var project struct {
		Services map[string]struct {
			Image       string            `json:"image"`
			Ports       []any             `json:"ports"`
			Environment map[string]string `json:"environment"`
			DependsOn   map[string]struct {
				Condition string `json:"condition"`
			} `json:"depends_on"`
		} `json:"services"`
	}
	require.NoError(t, json.Unmarshal(data, &project))
	t.Run("fork immutable image", func(t *testing.T) {
		require.Equal(t, "ghcr.io/dreamvm/one-hub@"+composeFixture["ONEHUB_IMAGE_DIGEST"], project.Services["one-hub"].Image)
	})
	t.Run("database private", func(t *testing.T) { require.Empty(t, project.Services["db"].Ports) })
	t.Run("explicit secrets", func(t *testing.T) {
		require.Equal(t, composeFixture["ONEHUB_SESSION_SECRET"], project.Services["one-hub"].Environment["SESSION_SECRET"])
		require.Equal(t, composeFixture["ONEHUB_USER_TOKEN_SECRET"], project.Services["one-hub"].Environment["USER_TOKEN_SECRET"])
	})
	t.Run("healthy dependencies", func(t *testing.T) {
		for _, name := range []string{"db", "redis"} {
			require.Equal(t, "service_healthy", project.Services["one-hub"].DependsOn[name].Condition)
		}
	})
}

func TestComposeOptionalModes(t *testing.T) {
	for _, mysql := range []bool{false, true} {
		for _, redis := range []bool{false, true} {
			files := []string{"docker-compose.sqlite.yml"}
			if mysql {
				files = append(files, "docker-compose.mysql.yml")
			}
			if redis {
				files = append(files, "docker-compose.redis.yml")
			}
			t.Run(strings.Join(files, "+"), func(t *testing.T) {
				missing := ""
				if !mysql {
					missing = "ONEHUB_MYSQL_PASSWORD,ONEHUB_MYSQL_ROOT_PASSWORD"
				}
				data, err := composeConfig(t, files, missing)
				require.NoError(t, err)
				var config struct {
					Services map[string]struct {
						Image       string            `json:"image"`
						Environment map[string]string `json:"environment"`
						Ports       []any             `json:"ports"`
						DependsOn   map[string]struct {
							Condition string `json:"condition"`
						} `json:"depends_on"`
						Healthcheck struct {
							Test []string `json:"test"`
						} `json:"healthcheck"`
					} `json:"services"`
				}
				require.NoError(t, json.Unmarshal(data, &config))
				app := config.Services["one-hub"]
				require.Equal(t, "ghcr.io/dreamvm/one-hub@"+composeFixture["ONEHUB_IMAGE_DIGEST"], app.Image)
				require.Len(t, app.Healthcheck.Test, 2)
				for service, enabled := range map[string]bool{"db": mysql, "redis": redis} {
					_, exists := config.Services[service]
					require.Equal(t, enabled, exists)
					if enabled {
						require.Empty(t, config.Services[service].Ports)
						require.Contains(t, config.Services[service].Image, "@sha256:")
						require.Equal(t, "service_healthy", app.DependsOn[service].Condition)
					} else {
						require.NotContains(t, app.DependsOn, service)
					}
				}
				if mysql {
					require.Equal(t, "oneapi:"+composeFixture["ONEHUB_MYSQL_PASSWORD"]+"@tcp(db:3306)/one-api", app.Environment["SQL_DSN"])
				} else {
					require.NotContains(t, app.Environment, "SQL_DSN")
				}
				if redis {
					require.Equal(t, "redis://redis:6379", app.Environment["REDIS_CONN_STRING"])
				} else {
					require.NotContains(t, app.Environment, "REDIS_CONN_STRING")
				}
			})
		}
	}
}
