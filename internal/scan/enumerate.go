//go:build windows

package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// Item is one file or directory the user can select.
type Item struct {
	Path    string
	Name    string
	IsDir   bool
	Reparse bool
	Class   Class
	Checked bool
	Default bool
	size    atomic.Int64
}

// Size returns the measured byte size. It is safe to call during measurement.
func (it *Item) Size() int64 {
	if it == nil {
		return 0
	}
	return it.size.Load()
}

// Group is a set of temp paths that share one naming pattern.
type Group struct {
	Class Class
	Items []*Item
}

// SelectedCount is the number of checked items in the group.
func (g *Group) SelectedCount() int {
	if g == nil {
		return 0
	}
	n := 0
	for _, it := range g.Items {
		if it.Checked {
			n++
		}
	}
	return n
}

// TotalSize sums measured sizes in the group.
func (g *Group) TotalSize() int64 {
	if g == nil {
		return 0
	}
	var n int64
	for _, it := range g.Items {
		n += it.Size()
	}
	return n
}

// AllChecked reports whether every item in the group is checked.
func (g *Group) AllChecked() bool {
	if g == nil || len(g.Items) == 0 {
		return false
	}
	for _, it := range g.Items {
		if !it.Checked {
			return false
		}
	}
	return true
}

// Result is a finished enumeration of one or more temp roots.
type Result struct {
	Roots  []string
	Groups []*Group
	Items  []*Item
}

// Callbacks receives English progress from a scan.
type Callbacks struct {
	Log        func(string)
	Enumerated func(dirs, items int)
}

// log writes an English progress line when a logger is set.
func (c Callbacks) log(format string, args ...any) {
	if c.Log == nil {
		return
	}
	c.Log(fmt.Sprintf(format, args...))
}

// DefaultRoots returns the user temp directory and Windows\Temp.
func DefaultRoots() []string {
	windir := os.Getenv("SystemRoot")
	if windir == "" {
		windir = os.Getenv("WINDIR")
	}
	roots := []string{os.TempDir(), os.Getenv("TEMP"), os.Getenv("TMP")}
	if windir != "" {
		roots = append(roots, filepath.Join(windir, "Temp"))
	}
	return normalizeRoots(roots)
}

// Enumerate lists temp entries and merges names that share a pattern.
// Nested Go build, gopls, and go mod directories are lifted into those groups.
func Enumerate(roots []string, cb Callbacks) *Result {
	roots = normalizeRoots(roots)
	if len(roots) == 0 {
		roots = DefaultRoots()
	}
	items := make([]*Item, 0, 1024)
	scanned := 0
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			cb.log("skip root %s: %v", root, err)
			continue
		}
		cb.log("scan root %s entries=%d", root, len(entries))
		for _, entry := range entries {
			emit(root, entry.Name(), 0, false, &items, &scanned, cb)
		}
	}
	return &Result{
		Roots:  roots,
		Items:  items,
		Groups: groupItems(items),
	}
}

// emit adds one temp entry, splitting containers that hold compiler directories.
func emit(parent, name string, depth int, inside bool, items *[]*Item, scanned *int, cb Callbacks) {
	path := filepath.Join(parent, name)
	info, err := os.Lstat(path)
	if err != nil {
		cb.log("skip %s: %v", path, err)
		return
	}
	*scanned++
	if *scanned%200 == 0 && cb.Enumerated != nil {
		cb.Enumerated(*scanned, len(*items))
	}
	if reparsePoint(info) || !info.IsDir() {
		*items = append(*items, newItem(path, name, info))
		return
	}
	if bulkPrefix(name) || Classify(name).Leaf || depth >= 4 {
		*items = append(*items, newItem(path, name, info))
		return
	}
	children, err := os.ReadDir(path)
	if err != nil {
		cb.log("cannot read %s: %v", path, err)
		*items = append(*items, newItem(path, name, info))
		return
	}
	if !shouldSplit(name, children, inside) {
		*items = append(*items, newItem(path, name, info))
		return
	}
	if len(children) > 400 && !isContainer(name) {
		cb.log("keep large directory as one item: %s children=%d", path, len(children))
		*items = append(*items, newItem(path, name, info))
		return
	}
	if len(children) == 0 {
		*items = append(*items, newItem(path, name, info))
		return
	}
	nextInside := inside || isContainer(name)
	for _, child := range children {
		emit(path, child.Name(), depth+1, nextInside, items, scanned, cb)
	}
}

// shouldSplit reports whether a directory's children should be listed on their own.
func shouldSplit(name string, children []os.DirEntry, inside bool) bool {
	if isContainer(name) {
		return true
	}
	if inside && isHexName(name) {
		return true
	}
	for _, child := range children {
		if child.IsDir() && compilerChild(child.Name()) {
			return true
		}
	}
	return false
}

// newItem builds one selectable temp entry from an lstat result.
func newItem(path, name string, info os.FileInfo) *Item {
	it := &Item{
		Path:    path,
		Name:    name,
		IsDir:   info.IsDir() && !reparsePoint(info),
		Reparse: reparsePoint(info),
		Class:   Classify(name),
	}
	if !it.IsDir && !it.Reparse {
		it.size.Store(info.Size())
	}
	return it
}

// groupItems merges items that share a class and applies default checkboxes.
func groupItems(items []*Item) []*Group {
	counts := make(map[string]int, len(items))
	for _, it := range items {
		counts[it.Class.ID]++
	}
	order := make([]string, 0)
	byID := make(map[string]*Group)
	for _, it := range items {
		it.Default = it.Class.DefaultChecked(counts[it.Class.ID]) && !it.Reparse
		it.Checked = it.Default
		g := byID[it.Class.ID]
		if g == nil {
			g = &Group{Class: it.Class}
			byID[it.Class.ID] = g
			order = append(order, it.Class.ID)
		}
		g.Items = append(g.Items, it)
	}
	groups := make([]*Group, 0, len(order))
	for _, id := range order {
		g := byID[id]
		sort.Slice(g.Items, func(i, j int) bool {
			return strings.ToLower(g.Items[i].Name) < strings.ToLower(g.Items[j].Name)
		})
		groups = append(groups, g)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		ai := groups[i].Items[0].Default
		aj := groups[j].Items[0].Default
		if ai != aj {
			return ai
		}
		return groups[i].Class.Title < groups[j].Class.Title
	})
	return groups
}

// reparsePoint reports a symlink, junction, or other reparse point.
func reparsePoint(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	attr, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || attr == nil {
		return false
	}
	return attr.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// Measure fills directory sizes. File sizes are already known.
func Measure(items []*Item, progress func(done, total int)) {
	pending := make([]*Item, 0)
	for _, it := range items {
		if it.IsDir && !it.Reparse {
			pending = append(pending, it)
		}
	}
	total := len(items)
	known := total - len(pending)
	if progress != nil {
		progress(known, total)
	}
	if len(pending) == 0 {
		return
	}
	workers := 4
	if len(pending) < workers {
		workers = len(pending)
	}
	jobs := make(chan *Item)
	var done atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range jobs {
				it.size.Store(dirSize(it.Path))
				n := int(done.Add(1)) + known
				if progress != nil && (n%25 == 0 || n == total) {
					progress(n, total)
				}
			}
		}()
	}
	for _, it := range pending {
		jobs <- it
	}
	close(jobs)
	wg.Wait()
	if progress != nil {
		progress(total, total)
	}
}

// dirSize sums regular files under root and does not follow reparse points.
func dirSize(root string) int64 {
	var total int64
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
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}
