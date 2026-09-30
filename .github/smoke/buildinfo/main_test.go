package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifyRealBinaryAndRejectMismatches(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	binary := filepath.Join(dir, "fixture")
	if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", binary, source)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	for _, test := range []struct {
		name, path, version, mainPath, goos, goarch, cgo string
		valid                                            bool
	}{
		{"normal stripped binary", binary, runtime.Version(), "command-line-arguments", runtime.GOOS, runtime.GOARCH, "0", true},
		{"old compiler", binary, "go1.25.0", "command-line-arguments", runtime.GOOS, runtime.GOARCH, "0", false},
		{"wrong application", binary, runtime.Version(), "one-api", runtime.GOOS, runtime.GOARCH, "0", false},
		{"wrong OS", binary, runtime.Version(), "command-line-arguments", "invalid", runtime.GOARCH, "0", false},
		{"wrong architecture", binary, runtime.Version(), "command-line-arguments", runtime.GOOS, "invalid", "0", false},
		{"CGO disabled", binary, runtime.Version(), "command-line-arguments", runtime.GOOS, runtime.GOARCH, "1", false},
		{"not a binary", source, runtime.Version(), "command-line-arguments", runtime.GOOS, runtime.GOARCH, "0", false},
		{"missing binary", filepath.Join(dir, "missing"), runtime.Version(), "command-line-arguments", runtime.GOOS, runtime.GOARCH, "0", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := verify(test.path, test.version, test.mainPath, test.goos, test.goarch, test.cgo)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}
