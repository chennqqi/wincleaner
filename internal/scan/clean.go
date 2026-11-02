//go:build windows

package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Job is one selected deletion that no longer shares state with the UI.
type Job struct {
	Path    string
	IsDir   bool
	Reparse bool
	Size    int64
}

// Stats is the outcome of a cleanup.
type Stats struct {
	Deleted int
	Failed  int
	Freed   int64
	Errors  []string
}

// JobsFrom copies the checked items. Call it on the UI thread before cleaning.
func JobsFrom(items []*Item) []Job {
	out := make([]Job, 0)
	for _, it := range items {
		if it == nil || !it.Checked {
			continue
		}
		out = append(out, Job{
			Path:    it.Path,
			IsDir:   it.IsDir,
			Reparse: it.Reparse,
			Size:    it.Size(),
		})
	}
	return out
}

// Clean deletes jobs that are strictly inside roots.
func Clean(jobs []Job, roots []string, logf func(string), progress func(done, total int)) Stats {
	var stats Stats
	if logf == nil {
		logf = func(string) {}
	}
	for i, job := range jobs {
		if progress != nil {
			progress(i, len(jobs))
		}
		if job.Path == "" || !WithinRoots(job.Path, roots) {
			stats.Failed++
			msg := fmt.Sprintf("refuse path outside temp: %s", job.Path)
			logf(msg)
			stats.addError(msg)
			continue
		}
		if err := removeJob(job); err != nil {
			stats.Failed++
			msg := fmt.Sprintf("delete failed path=%s err=%v", job.Path, err)
			logf(msg)
			stats.addError(msg)
			continue
		}
		stats.Deleted++
		stats.Freed += job.Size
	}
	if progress != nil {
		progress(len(jobs), len(jobs))
	}
	logf(fmt.Sprintf("clean finished deleted=%d failed=%d bytes=%d", stats.Deleted, stats.Failed, stats.Freed))
	return stats
}

// addError keeps the first failures for the result dialog.
func (s *Stats) addError(msg string) {
	if len(s.Errors) >= 20 {
		return
	}
	s.Errors = append(s.Errors, msg)
}

// removeJob deletes one temp file or directory without following a reparse point.
func removeJob(job Job) error {
	if job.Reparse {
		return os.Remove(job.Path)
	}
	if !job.IsDir {
		info, err := os.Lstat(job.Path)
		if err != nil {
			return err
		}
		clearReadonly(job.Path, info)
		return os.Remove(job.Path)
	}
	clearTreeReadonly(job.Path)
	return os.RemoveAll(job.Path)
}

// clearTreeReadonly clears the read-only bit so Windows can delete temp files.
// Reparse points are left untouched so their targets are not modified.
func clearTreeReadonly(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if reparsePoint(info) {
			if path != root && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		clearReadonly(path, info)
		return nil
	})
}

// clearReadonly removes the read-only attribute so Windows can delete the file.
func clearReadonly(path string, info os.FileInfo) {
	attr, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || attr == nil {
		return
	}
	if attr.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY == 0 {
		return
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_ = syscall.SetFileAttributes(p, attr.FileAttributes&^syscall.FILE_ATTRIBUTE_READONLY)
}
