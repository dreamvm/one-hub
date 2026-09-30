// Command buildinfo verifies the compiler identity of an extracted candidate.
package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"flag"
	"fmt"
	"io"
	"os"
)

func verify(path, version, mainPath, goos, goarch, cgo string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read candidate build metadata: %w", err)
	}
	if info.GoVersion != version || info.Path != mainPath {
		return fmt.Errorf("unexpected candidate identity: Go=%q main=%q", info.GoVersion, info.Path)
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	for key, expected := range map[string]string{"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": cgo} {
		if settings[key] != expected {
			return fmt.Errorf("unexpected candidate %s=%q, want %q", key, settings[key], expected)
		}
	}
	return nil
}

func main() {
	binary := flag.String("binary", "", "extracted candidate executable")
	version := flag.String("go-version", "", "required exact Go compiler version")
	goarch := flag.String("goarch", "", "required target architecture")
	flag.Parse()
	if *binary == "" || *version == "" || *goarch == "" {
		fmt.Fprintln(os.Stderr, "binary, go-version and goarch are required")
		os.Exit(1)
	}
	if err := verify(*binary, *version, "one-api", "linux", *goarch, "1"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	file, err := os.Open(*binary)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("CANDIDATE_BUILD_INFO Go=%s main=one-api GOOS=linux GOARCH=%s CGO_ENABLED=1 SHA256=%x\n", *version, *goarch, hash.Sum(nil))
}
