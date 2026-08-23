package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache manages file-based JSON caching following XDG Base Directory specification in production.
type Cache struct {
	dir string
}

// ResolveCacheDir determines the active cache directory.
// Uses .tmp for local dev (if FETCH_MIX_DEV=1 or .tmp directory exists in cwd).
// Uses XDG_CACHE_HOME / os.UserCacheDir() ("fetch-mix") in production.
func ResolveCacheDir(customDir ...string) (string, error) {
	if len(customDir) > 0 && customDir[0] != "" {
		return customDir[0], nil
	}

	envDev := strings.TrimSpace(os.Getenv("FETCH_MIX_DEV"))
	isDev := envDev == "1" || strings.ToLower(envDev) == "true"

	if isDev {
		return ".tmp", nil
	}

	// If local .tmp folder exists in current working directory, prefer it for local dev workflow
	if fi, err := os.Stat(".tmp"); err == nil && fi.IsDir() {
		return ".tmp", nil
	}

	// Production: XDG Base Directory specification ($XDG_CACHE_HOME / os.UserCacheDir())
	userCache, err := os.UserCacheDir()
	if err != nil {
		homeDir, hErr := os.UserHomeDir()
		if hErr != nil {
			return ".tmp", nil
		}
		userCache = filepath.Join(homeDir, ".cache")
	}

	return filepath.Join(userCache, "fetch-mix"), nil
}

// New creates a new Cache instance.
func New(dir ...string) (*Cache, error) {
	cacheDir, err := ResolveCacheDir(dir...)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir %q: %w", cacheDir, err)
	}

	return &Cache{dir: cacheDir}, nil
}

// Dir returns the active cache directory path.
func (c *Cache) Dir() string {
	return c.dir
}

// HashKey generates a short 16-character SHA-256 hash string for key filenames.
func HashKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// Get reads and unmarshals JSON cached data for key filename.
func (c *Cache) Get(key string, target interface{}) bool {
	filePath := filepath.Join(c.dir, key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false
	}

	return true
}

// Put marshals and writes data to a cache file.
func (c *Cache) Put(key string, data interface{}) error {
	filePath := filepath.Join(c.dir, key)
	bytesData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	if err := os.WriteFile(filePath, bytesData, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Delete removes a cache entry file if it exists.
func (c *Cache) Delete(key string) error {
	filePath := filepath.Join(c.dir, key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file %q: %w", filePath, err)
	}
	return nil
}

// Path returns the full path for a key.
func (c *Cache) Path(key string) string {
	return filepath.Join(c.dir, key)
}
