package deps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/cache"
)

// IsAgentMode checks if AGENT=1 or AGENT=true environment variable is set.
func IsAgentMode() bool {
	val := strings.TrimSpace(os.Getenv("AGENT"))
	return val == "1" || strings.EqualFold(val, "true")
}

type Dependency struct {
	Name       string
	MinVersion string
	InstallURL string
}

var RequiredDependencies = []Dependency{
	{
		Name:       "fetch-track",
		MinVersion: "0.1.0",
		InstallURL: "https://github.com/alexgorbatchev/fetch-track-cli",
	},
	{
		Name:       "yt-dlp",
		MinVersion: "2024.08.01",
		InstallURL: "https://github.com/yt-dlp/yt-dlp#installation",
	},
	{
		Name:       "ffmpeg",
		MinVersion: "4.4",
		InstallURL: "https://ffmpeg.org/download.html",
	},
	{
		Name:       "firecrawl",
		MinVersion: "1.0.0",
		InstallURL: "https://github.com/alexgorbatchev/firecrawl-cli",
	},
}

type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func DefaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

type DependencyReport struct {
	Name            string `json:"name"`
	MinVersion      string `json:"minVersion"`
	DetectedVersion string `json:"detectedVersion"`
	Installed       bool   `json:"installed"`
	Satisfied       bool   `json:"satisfied"`
	Error           string `json:"error,omitempty"`
}

func CheckDependencies(ctx context.Context, cacheInst ...*cache.Cache) error {
	var c *cache.Cache
	if len(cacheInst) > 0 {
		c = cacheInst[0]
	} else {
		c, _ = cache.New()
	}
	return CheckDependenciesWithRunner(ctx, DefaultRunner, c, RequiredDependencies...)
}

func CheckDependenciesWithRunner(ctx context.Context, runner CommandRunner, c *cache.Cache, deps ...Dependency) error {
	if len(deps) == 0 {
		deps = RequiredDependencies
	}
	_, err := VerifyDependenciesWithRunner(ctx, runner, c, deps...)
	return err
}

func VerifyDependencies(ctx context.Context, cacheInst ...*cache.Cache) ([]DependencyReport, error) {
	var c *cache.Cache
	if len(cacheInst) > 0 {
		c = cacheInst[0]
	} else {
		c, _ = cache.New()
	}
	return VerifyDependenciesWithRunner(ctx, DefaultRunner, c, RequiredDependencies...)
}

func VerifyDependenciesWithRunner(ctx context.Context, runner CommandRunner, c *cache.Cache, deps ...Dependency) ([]DependencyReport, error) {
	reports := make([]DependencyReport, len(deps))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	isAgent := IsAgentMode()

	for i, dep := range deps {
		wg.Add(1)
		go func(idx int, dep Dependency) {
			defer wg.Done()

			report := DependencyReport{
				Name:       dep.Name,
				MinVersion: dep.MinVersion,
			}

			var out []byte
			var err error

			var cachedStr string
			if c != nil && c.Get(fmt.Sprintf("deps_%s.json", dep.Name), &cachedStr) && cachedStr != "" {
				out = []byte(cachedStr)
			} else {
				var versionArgs []string
				if dep.Name == "yt-dlp" || dep.Name == "fetch-track" || dep.Name == "firecrawl" {
					versionArgs = []string{"--version"}
				} else {
					versionArgs = []string{"-version"}
				}

				out, err = runner(ctx, dep.Name, versionArgs...)
				if err == nil && len(out) > 0 && c != nil {
					_ = c.Put(fmt.Sprintf("deps_%s.json", dep.Name), string(out))
				}
			}

			if err != nil {
				if isNotFound(err) {
					report.Installed = false
					report.Satisfied = false
					if isAgent {
						report.Error = fmt.Sprintf("%s is missing in $PATH. Ask user for confirmation to install %s version %s or newer.", dep.Name, dep.Name, dep.MinVersion)
					} else {
						report.Error = fmt.Sprintf("%s is missing in $PATH, please install %s (%s)", dep.Name, dep.Name, dep.InstallURL)
					}
				} else {
					report.Installed = false
					report.Satisfied = false
					report.Error = fmt.Sprintf("failed to check %s version: %v", dep.Name, err)
				}
			} else {
				report.Installed = true
				versionStr := ParseVersionOutput(dep.Name, string(out))
				report.DetectedVersion = versionStr

				if versionStr == "" {
					report.Satisfied = false
					report.Error = fmt.Sprintf("could not parse %s version output", dep.Name)
				} else if compErr := CompareVersions(versionStr, dep.MinVersion); compErr != nil {
					report.Satisfied = false
					if isAgent {
						report.Error = fmt.Sprintf("%s version %s in $PATH is outdated. Ask user for confirmation to update %s to version %s or newer.", dep.Name, versionStr, dep.Name, dep.MinVersion)
					} else {
						report.Error = fmt.Sprintf("%s in $PATH must be version %s or newer", dep.Name, dep.MinVersion)
					}
				} else {
					report.Satisfied = true
				}
			}

			mu.Lock()
			reports[idx] = report
			if !report.Satisfied && firstErr == nil {
				firstErr = errors.New(report.Error)
			}
			mu.Unlock()
		}(i, dep)
	}

	wg.Wait()
	return reports, firstErr
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "executable file not found") || strings.Contains(errStr, "not found")
}

func ParseVersionOutput(depName, output string) string {
	line := strings.TrimSpace(output)
	if idx := strings.Index(line, "\n"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	switch depName {
	case "yt-dlp":
		re := regexp.MustCompile(`\b(\d{4}\.\d{2}\.\d{2}(?:\.\d+)?)\b`)
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			return match[1]
		}
		return line
	case "ffmpeg":
		re := regexp.MustCompile(`(?:ffmpeg)\s+version\s+(?:n)?([^\s]+)`)
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			return match[1]
		}
	case "fetch-track", "firecrawl":
		if line == "dev" || strings.HasPrefix(line, "v") {
			return strings.TrimPrefix(line, "v")
		}
		return line
	}
	return line
}

func CompareVersions(actualStr, minStr string) error {
	if actualStr == "dev" || strings.HasPrefix(actualStr, "N-") || strings.HasPrefix(actualStr, "git-") {
		return nil
	}

	actualParts := parseVersionParts(actualStr)
	minParts := parseVersionParts(minStr)

	if len(actualParts) == 0 {
		return nil
	}

	for i := 0; i < len(minParts); i++ {
		actVal := 0
		if i < len(actualParts) {
			actVal = actualParts[i]
		}
		minVal := minParts[i]

		if actVal > minVal {
			return nil
		}
		if actVal < minVal {
			return fmt.Errorf("version below minimum requirement")
		}
	}

	return nil
}

func parseVersionParts(v string) []int {
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}

	rawParts := strings.Split(v, ".")
	var parts []int
	for _, p := range rawParts {
		digits := ""
		for _, ch := range p {
			if ch >= '0' && ch <= '9' {
				digits += string(ch)
			} else {
				break
			}
		}
		if digits == "" {
			break
		}
		val, err := strconv.Atoi(digits)
		if err != nil {
			break
		}
		parts = append(parts, val)
	}
	return parts
}
