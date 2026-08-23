package cache

import (
	"os"
	"testing"
)

func TestCache_GetPutDelete(t *testing.T) {
	tempDir := t.TempDir()
	c, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
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

	// Get after Delete should return false
	if c.Get(key, &got) {
		t.Errorf("Get() after Delete returned true, want false")
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
	os.Unsetenv("FETCH_MIX_DEV")
}
