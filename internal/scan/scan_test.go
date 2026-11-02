//go:build windows

package scan

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestEnumerateGroupsNestedCompilerTemps checks merged groups and a safe delete.
func TestEnumerateGroupsNestedCompilerTemps(t *testing.T) {
	root := t.TempDir()
	hex := "0ad483ac3416dc6f3adca0a24af5e5da"
	mustMkdir(t, filepath.Join(root, "cursor-sandbox-cache", hex, "go-build"))
	mustMkdir(t, filepath.Join(root, "cursor-sandbox-cache", hex, "go-mod"))
	mustMkdir(t, filepath.Join(root, "cursor-sandbox-cache", hex, "npm"))
	mustMkdir(t, filepath.Join(root, "cursor-sandbox-cache", hex, "playwright"))
	mustWrite(t, filepath.Join(root, "cursor-sandbox-cache", hex, "go-build", "a.o"), []byte("abcd"))
	mustMkdir(t, filepath.Join(root, "gopls-10.1"))
	mustMkdir(t, filepath.Join(root, "gopls-11.1"))
	mustMkdir(t, filepath.Join(root, "aurivex-foo-gocache"))
	mustMkdir(t, filepath.Join(root, "aurivex-bar-gocache"))
	mustMkdir(t, filepath.Join(root, "wrapper", "go-build"))
	mustWrite(t, filepath.Join(root, "wrapper", "keep.txt"), []byte("k"))
	mustWrite(t, filepath.Join(root, "notes.json"), []byte("{}"))
	mustWrite(t, filepath.Join(root, "pic.png"), []byte("png"))
	mustWrite(t, filepath.Join(root, "alone-unique.txt"), []byte("u"))
	mustMkdir(t, filepath.Join(root, "foo-1"))
	mustMkdir(t, filepath.Join(root, "foo-2"))

	got := Enumerate([]string{root}, Callbacks{})
	build := findGroup(t, got, "go-build")
	if len(build.Items) != 4 {
		t.Fatalf("go-build items=%d, want the sandbox dir, two gocaches, and wrapper/go-build", len(build.Items))
	}
	if !build.AllChecked() {
		t.Fatal("go build temps should start checked")
	}
	if findGroup(t, got, "gopls").SelectedCount() != 2 {
		t.Fatal("both gopls dirs should be checked")
	}
	if len(findGroup(t, got, "go-mod").Items) != 1 || !findGroup(t, got, "go-mod").AllChecked() {
		t.Fatal("nested go-mod should be its own checked item")
	}
	if findGroup(t, got, "node").SelectedCount() != 1 {
		t.Fatal("nested npm should join the node group")
	}
	play := findGroup(t, got, "one:playwright")
	if play.AllChecked() || len(play.Items) != 1 {
		t.Fatal("playwright stays optional")
	}
	if findGroup(t, got, "config:.json").AllChecked() {
		t.Fatal("json stays unchecked")
	}
	if findGroup(t, got, "pat:foo-*").SelectedCount() != 2 {
		t.Fatal("repeated foo-* dirs should be checked")
	}
	for _, it := range got.Items {
		base := filepath.Base(it.Path)
		if base == "cursor-sandbox-cache" || base == hex || base == "wrapper" {
			t.Fatalf("container was listed as its own item: %s", it.Path)
		}
	}
	Measure(got.Items, nil)
	if findGroup(t, got, "go-build").TotalSize() < 4 {
		t.Fatalf("go-build size=%d", findGroup(t, got, "go-build").TotalSize())
	}

	keep := filepath.Join(root, "notes.json")
	ro := filepath.Join(root, "locked.tmp")
	mustWrite(t, ro, []byte("ro"))
	setReadonly(t, ro)
	// The readonly file was created after enumeration. Add it as a selected job.
	jobs := JobsFrom(got.Items)
	jobs = append(jobs, Job{Path: ro, IsDir: false, Size: 2})
	outside := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(outside, []byte("stay"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs = append(jobs, Job{Path: outside, IsDir: false, Size: 4})

	stats := Clean(jobs, got.Roots, func(string) {}, nil)
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("unchecked json should remain")
	}
	playPath := filepath.Join(root, "cursor-sandbox-cache", hex, "playwright")
	if _, err := os.Stat(playPath); err != nil {
		t.Fatalf("unchecked playwright directory should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "wrapper", "keep.txt")); err != nil {
		t.Fatal("unchecked file inside a split directory should remain")
	}
	if _, err := os.Stat(filepath.Join(root, "cursor-sandbox-cache", hex, "go-build")); !os.IsNotExist(err) {
		t.Fatalf("go-build should be deleted: %v", err)
	}
	if _, err := os.Stat(ro); !os.IsNotExist(err) {
		t.Fatalf("read-only temp file should be deleted: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("path outside the scanned root must remain")
	}
	if stats.Failed != 1 {
		t.Fatalf("failed=%d, want the outside path only", stats.Failed)
	}
}

// findGroup returns the group with id or fails the test.
func findGroup(t *testing.T, result *Result, id string) *Group {
	t.Helper()
	for _, g := range result.Groups {
		if g.Class.ID == id {
			return g
		}
	}
	t.Fatalf("missing group %s", id)
	return nil
}

// mustMkdir creates a directory for a test.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// mustWrite creates a file for a test.
func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// setReadonly marks a file read-only so cleanup must clear the attribute.
func setReadonly(t *testing.T, path string) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
}
