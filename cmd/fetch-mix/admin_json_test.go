package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/godeps/v2"
)

func TestAdminJSONDependencies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("AGENT", "0")
	original := deps.RequiredDependencies
	t.Cleanup(func() { deps.RequiredDependencies = original })
	calls := 0
	deps.RequiredDependencies = []deps.Dependency{{Name: "admin-fixture", MinVersion: "1.0.0", Installer: godeps.InstallerFunc(func(ctx context.Context, dir string) error {
		calls++
		return os.WriteFile(filepath.Join(dir, "admin-fixture"), []byte("#!/bin/sh\nprintf '9.0.0\\n'\n"), 0755)
	})}}
	run := func(args ...string) (map[string]json.RawMessage, error) {
		t.Helper()
		command := newRootCmd()
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&bytes.Buffer{})
		command.SetArgs(append([]string{"--json", "deps"}, args...))
		err := command.Execute()
		var result map[string]json.RawMessage
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("expected only JSON, got %q: %v", out.String(), decodeErr)
		}
		var reports []deps.DependencyReport
		if e := json.Unmarshal(result["dependencies"], &reports); e != nil || len(reports) != 1 {
			t.Fatalf("missing native reports: %s (%v)", out.String(), e)
		}
		if string(result["changed"]) == "null" {
			t.Fatal("changed must be an array")
		}
		return result, err
	}
	result, err := run()
	if err == nil || string(result["status"]) != `"error"` {
		t.Fatalf("missing fixture must fail: %s %v", result, err)
	}
	for _, args := range [][]string{{"install"}, {"update"}, {"install", "admin-fixture"}, {"update", "admin-fixture"}} {
		result, err = run(args...)
		if err != nil || string(result["status"]) != `"ok"` || string(result["changed"]) != `["admin-fixture"]` {
			t.Fatalf("mutation %v: %s %v", args, result, err)
		}
	}
	if calls != 4 {
		t.Fatalf("native installer calls: %d", calls)
	}
	result, err = run("install")
	if err != nil || string(result["changed"]) != "[]" {
		t.Fatalf("satisfied install: %s %v", result, err)
	}
	for _, action := range []string{"install", "update"} {
		result, err = run(action, "unknown")
		if err == nil || string(result["status"]) != `"error"` {
			t.Fatalf("unknown dependency: %s %v", result, err)
		}
	}
	deps.RequiredDependencies[0].Installer = godeps.InstallerFunc(func(context.Context, string) error { return errors.New("fixture failure") })
	result, err = run("update")
	if err == nil || string(result["status"]) != `"error"` {
		t.Fatalf("failed update: %s %v", result, err)
	}
}

func TestAdminJSONInspectsCurrentManagedDependencies(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(map[bool]string{false: "path", true: "managed"}[managed], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("FETCH_MIX_DEV", "0")
			t.Chdir(t.TempDir())
			externalDir := t.TempDir()
			t.Setenv("PATH", externalDir)
			original := deps.RequiredDependencies
			t.Cleanup(func() { deps.RequiredDependencies = original })
			const name = "inspection-fixture"
			deps.RequiredDependencies = []deps.Dependency{{Name: name, MinVersion: "1.0.0"}}
			writeDependencyVersion(t, externalDir, name, "0.1.0")
			dir := externalDir
			if managed {
				var err error
				dir, err = deps.EnsureManagedBinDir()
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, version := range []string{"1.0.0", "2.0.0", "0.5.0"} {
				writeDependencyVersion(t, dir, name, version)
				// Each command starts with the same inherited PATH, as separate CLI invocations do.
				t.Setenv("PATH", externalDir)
				command := newRootCmd()
				var out bytes.Buffer
				command.SetOut(&out)
				command.SetErr(&bytes.Buffer{})
				command.SetArgs([]string{"--json", "dependencies", "verify"})
				err := command.Execute()
				var result dependencyJSONResult
				if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
					t.Fatalf("expected JSON, got %q: %v", out.String(), decodeErr)
				}
				// An outdated managed copy never replaces another copy on PATH.
				wantVersion := version
				if managed && version == "0.5.0" {
					wantVersion = "0.1.0"
				}
				if len(result.Dependencies) != 1 || result.Dependencies[0].DetectedVersion != wantVersion {
					t.Fatalf("current version %s: %+v (%v)", wantVersion, result, err)
				}
				wantSatisfied := version != "0.5.0"
				if result.Dependencies[0].Satisfied != wantSatisfied || (err == nil) != wantSatisfied {
					t.Fatalf("current requirement status for %s: %+v (%v)", version, result, err)
				}
			}
		})
	}
}

func TestAdminJSONUpgrade(t *testing.T) {
	for _, tc := range []struct {
		version string
		err     error
		status  string
	}{{"9.0.0", nil, "ok"}, {"", errors.New("already at the latest version"), "current"}, {"", errors.New("fixture failure"), "error"}} {
		t.Run(tc.status, func(t *testing.T) {
			original := adminUpgradeSelf
			adminUpgradeSelf = func(context.Context, string) (string, error) { return tc.version, tc.err }
			t.Cleanup(func() { adminUpgradeSelf = original })
			command := newRootCmd()
			var out bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs([]string{"--json", "self", "upgrade"})
			err := command.Execute()
			var result struct {
				Status  string `json:"status"`
				Version string `json:"version"`
				Error   string `json:"error"`
			}
			if e := json.Unmarshal(out.Bytes(), &result); e != nil {
				t.Fatalf("expected only JSON, got %q: %v", out.String(), e)
			}
			if result.Status != tc.status || (err != nil) != (tc.status == "error") {
				t.Fatalf("upgrade: %+v %v", result, err)
			}
			if tc.status == "ok" && result.Version != "9.0.0" {
				t.Fatalf("version: %+v", result)
			}
			if tc.status == "error" && result.Error == "" {
				t.Fatal("missing error")
			}
		})
	}
}
