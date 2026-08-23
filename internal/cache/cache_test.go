package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCache_GetPutDelete(t *testing.T) {
	tempDir := t.TempDir()
	c, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if c.Dir() != tempDir {
		t.Errorf("c.Dir() = %q, want %q", c.Dir(), tempDir)
	}

	expectedPath := filepath.Join(tempDir, "test.json")
	if c.Path("test.json") != expectedPath {
		t.Errorf("c.Path() = %q, want %q", c.Path("test.json"), expectedPath)
	}

	key := "test_key.json"
	value := "hello world"

	// Initial Get should return false
	var got string
	if c.Get(key, &got) {
		t.Errorf("Get() before Put returned true, want false")
	}

	// Put value
	if err := c.Put(key, value); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Get value
	if !c.Get(key, &got) {
		t.Fatalf("Get() after Put returned false, want true")
	}
	if got != value {
		t.Errorf("Get() = %q, want %q", got, value)
	}

	// Delete key
	if err := c.Delete(key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Delete again (non-existent) should not error
	if err := c.Delete(key); err != nil {
		t.Errorf("Delete() on non-existent file error = %v", err)
	}

	// Get after Delete should return false
	if c.Get(key, &got) {
		t.Errorf("Get() after Delete returned true, want false")
	}

	// New with invalid directory (path is a regular file)
	filePath := filepath.Join(tempDir, "file_not_dir")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filePath); err == nil {
		t.Errorf("New() with file path as dir should fail")
	}

	// Invalid JSON file Get
	badJSONFile := filepath.Join(tempDir, "bad.json")
	if err := os.WriteFile(badJSONFile, []byte("{invalid-json"), 0644); err != nil {
		t.Fatal(err)
	}
	var dummy map[string]string
	if c.Get("bad.json", &dummy) {
		t.Errorf("Get() on invalid JSON returned true, want false")
	}

	// Put with invalid data
	ch := make(chan int)
	if err := c.Put("bad_chan.json", ch); err == nil {
		t.Errorf("Put() with unmarshalable data should error")
	}

	// Put in read-only directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0555); err == nil {
		roCache, _ := New(readOnlyDir)
		_ = os.Chmod(readOnlyDir, 0444)
		if roCache != nil {
			_ = roCache.Put("ro.json", "data")
		}
		_ = os.Chmod(readOnlyDir, 0755)
	}
}

func TestHashKey(t *testing.T) {
	key := "https://www.youtube.com/watch?v=12345"
	h1 := HashKey(key)
	h2 := HashKey(key)
	if len(h1) != 16 {
		t.Errorf("HashKey length = %d, want 16", len(h1))
	}
	if h1 != h2 {
		t.Errorf("HashKey not deterministic: %q vs %q", h1, h2)
	}
}

func TestResolveCacheDir(t *testing.T) {
	custom := "/custom/cache/dir"
	resolved, err := ResolveCacheDir(custom)
	if err != nil {
		t.Fatalf("ResolveCacheDir(custom) error = %v", err)
	}
	if resolved != custom {
		t.Errorf("ResolveCacheDir(custom) = %q, want %q", resolved, custom)
	}

	os.Setenv("FETCH_MIX_DEV", "1")
	resolvedDev, err := ResolveCacheDir()
	if err != nil {
		t.Fatalf("ResolveCacheDir(dev) error = %v", err)
	}
	if resolvedDev != ".tmp" {
		t.Errorf("ResolveCacheDir(dev) = %q, want '.tmp'", resolvedDev)
	}

	os.Setenv("FETCH_MIX_DEV", "true")
	resolvedDevTrue, _ := ResolveCacheDir()
	if resolvedDevTrue != ".tmp" {
		t.Errorf("ResolveCacheDir(dev=true) = %q, want '.tmp'", resolvedDevTrue)
	}
	os.Unsetenv("FETCH_MIX_DEV")

	// Test production resolution when FETCH_MIX_DEV is not set and .tmp does not exist in cwd
	t.Run("prod_user_cache_dir", func(t *testing.T) {
		tempCwd := t.TempDir()
		t.Chdir(tempCwd)
		t.Setenv("FETCH_MIX_DEV", "0")
		t.Setenv("XDG_CACHE_HOME", tempCwd)
		resolvedProd, err := ResolveCacheDir()
		if err != nil {
			t.Fatalf("ResolveCacheDir() prod error = %v", err)
		}
		if !strings.HasSuffix(resolvedProd, "fetch-mix") {
			t.Errorf("ResolveCacheDir() prod = %q, want suffix 'fetch-mix'", resolvedProd)
		}
	})

	t.Run("prod_user_cache_dir_error_fallback", func(t *testing.T) {
		tempCwd := t.TempDir()
		t.Chdir(tempCwd)
		t.Setenv("FETCH_MIX_DEV", "0")
		t.Setenv("XDG_CACHE_HOME", "")
		t.Setenv("HOME", "")
		resolved, err := ResolveCacheDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved == "" {
			t.Errorf("expected non-empty fallback cache dir")
		}
	})
}
