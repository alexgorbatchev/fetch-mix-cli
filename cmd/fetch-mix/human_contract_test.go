package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/deps"
	"github.com/alexgorbatchev/godeps"
)

func TestHumanAndAgentDiscovery(t *testing.T) {
	for _, agent := range []string{"0", "1"} {
		t.Run(agent, func(t *testing.T) {
			t.Setenv("AGENT", agent)
			for _, args := range [][]string{{"ai", "list"}, {"ai", "inspect", "openai"}} {
				command := newRootCmd()
				command.SetArgs(args)
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
			}
			command := newRootCmd()
			var out bytes.Buffer
			command.SetOut(&out)
			if err := command.Usage(); err != nil || !strings.Contains(out.String(), "fetch-mix") {
				t.Fatalf("usage: %s %v", out.String(), err)
			}
		})
	}
}

func TestNativeDependencyInstallAndUpdateFixtures(t *testing.T) {
	original := deps.RequiredDependencies
	t.Cleanup(func() { deps.RequiredDependencies = original })
	for _, agent := range []string{"0", "1"} {
		for _, action := range []string{"install", "update"} {
			for _, named := range []bool{true, false} {
				t.Run(fmt.Sprintf("agent%s/%s/named%t", agent, action, named), func(t *testing.T) {
					t.Chdir(t.TempDir())
					t.Setenv("AGENT", agent)
					t.Setenv("PATH", t.TempDir())
					t.Setenv("HOME", t.TempDir())
					t.Setenv("XDG_CACHE_HOME", t.TempDir())
					t.Setenv("XDG_DATA_HOME", t.TempDir())
					calls := 0
					deps.RequiredDependencies = []deps.Dependency{{Name: "contract-fixture", MinVersion: "1.0.0", Installer: godeps.InstallerFunc(func(ctx context.Context, dir string) error {
						calls++
						return os.WriteFile(filepath.Join(dir, "contract-fixture"), []byte("#!/bin/sh\nprintf '9.0.0\\n'\n"), 0755)
					})}}
					args := []string{"dependencies", action}
					if named {
						args = append(args, "contract-fixture")
					}
					command := newRootCmd()
					command.SetArgs(args)
					if err := command.Execute(); err != nil || calls != 1 {
						t.Fatalf("fixture install: %d %v", calls, err)
					}
					command = newRootCmd()
					command.SetArgs([]string{"dependencies", "install"})
					if err := command.Execute(); err != nil || calls != 1 {
						t.Fatalf("satisfied dependency reinstalled: %d %v", calls, err)
					}
				})
			}
		}
	}
}
