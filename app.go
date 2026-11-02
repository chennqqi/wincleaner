//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/chennqqi/wincleaner/internal/scan"
)

// groupView is one merged pattern shown in the left list.
type groupView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
	Count    int    `json:"count"`
	Selected int    `json:"selected"`
	SizeText string `json:"size_text"`
	Checked  bool   `json:"checked"`
}

// itemView is one file or directory inside the selected group.
type itemView struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	SizeText string `json:"size_text"`
	Checked  bool   `json:"checked"`
}

// viewState is the snapshot drawn by the WebView.
type viewState struct {
	Busy       bool        `json:"busy"`
	Status     string      `json:"status"`
	Groups     []groupView `json:"groups"`
	Items      []itemView  `json:"items"`
	SelectedID string      `json:"selected_id"`
	Chosen     int         `json:"chosen"`
	ChosenText string      `json:"chosen_text"`
	Done       int         `json:"done"`
	Total      int         `json:"total"`
	Logs       []string    `json:"logs"`
}

// cleanResult is the summary shown after a cleanup.
type cleanResult struct {
	Empty     bool     `json:"empty"`
	Deleted   int      `json:"deleted"`
	Failed    int      `json:"failed"`
	FreedText string   `json:"freed_text"`
	Errors    []string `json:"errors"`
	Message   string   `json:"message"`
}

// App is the bound cleaner API for one window.
type App struct {
	ctx        context.Context
	mu         sync.Mutex
	result     *scan.Result
	selectedID string
	status     string
	logs       []string
	done       int
	total      int
	busy       bool
	lastEmit   time.Time
}

// NewApp prepares the cleaner before the window is shown.
func NewApp() *App {
	return &App{status: "准备扫描…"}
}

// startup stores the Wails context and starts the first scan.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	go a.Scan()
}

// Scan lists temp roots, measures sizes, and publishes the groups.
func (a *App) Scan() {
	if !a.begin() {
		return
	}
	defer a.end()
	a.mu.Lock()
	a.status = "正在扫描临时目录…"
	a.done, a.total = 0, 0
	a.mu.Unlock()
	a.publish()

	result := scan.Enumerate(nil, scan.Callbacks{
		Log: a.appendLog,
		Enumerated: func(dirs, items int) {
			a.mu.Lock()
			a.status = fmt.Sprintf("正在扫描，已查看 %d 个目录，发现 %d 项", dirs, items)
			a.mu.Unlock()
			a.touch()
		},
	})
	a.mu.Lock()
	a.applyResultLocked(result)
	a.status = fmt.Sprintf("正在统计大小，共 %d 个分组、%d 项", len(result.Groups), len(result.Items))
	a.mu.Unlock()
	a.publish()

	scan.Measure(result.Items, func(done, total int) {
		a.mu.Lock()
		a.done, a.total = done, total
		a.status = fmt.Sprintf("正在统计大小 %d / %d", done, total)
		a.mu.Unlock()
		if done == total {
			a.publish()
			return
		}
		a.touch()
	})

	a.mu.Lock()
	a.sortGroupsLocked()
	a.done, a.total = 0, 0
	a.refreshStatusLocked()
	groups, items := 0, 0
	if a.result != nil {
		groups = len(a.result.Groups)
		items = len(a.result.Items)
	}
	a.mu.Unlock()
	a.appendLog(fmt.Sprintf("scan finished groups=%d items=%d", groups, items))
}

// State returns the current list for the page load.
func (a *App) State() viewState {
	return a.snapshot()
}

// SelectGroup shows the files that belong to one pattern.
func (a *App) SelectGroup(id string) {
	a.mu.Lock()
	if a.findGroupLocked(id) == nil {
		a.mu.Unlock()
		return
	}
	a.selectedID = id
	a.mu.Unlock()
	a.publish()
}

// SetGroup checks or clears every item in one pattern.
func (a *App) SetGroup(id string, checked bool) {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return
	}
	g := a.findGroupLocked(id)
	if g == nil {
		a.mu.Unlock()
		return
	}
	for _, it := range g.Items {
		it.Checked = checked
	}
	a.selectedID = id
	a.refreshStatusLocked()
	a.mu.Unlock()
	a.publish()
}

// SetItem checks or clears one file inside a group.
func (a *App) SetItem(id string, index int, checked bool) {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return
	}
	g := a.findGroupLocked(id)
	if g == nil || index < 0 || index >= len(g.Items) {
		a.mu.Unlock()
		return
	}
	g.Items[index].Checked = checked
	a.refreshStatusLocked()
	a.mu.Unlock()
	a.publish()
}

// SelectDefault restores the checkbox chosen by the classifier.
func (a *App) SelectDefault() {
	a.setAll(func(it *scan.Item) bool { return it.Default })
}

// SelectAll checks every discovered item.
func (a *App) SelectAll() {
	a.setAll(func(*scan.Item) bool { return true })
}

// SelectNone clears every checkbox.
func (a *App) SelectNone() {
	a.setAll(func(*scan.Item) bool { return false })
}

// Invert flips every checkbox.
func (a *App) Invert() {
	a.setAll(func(it *scan.Item) bool { return !it.Checked })
}

// Clean deletes the currently checked temp paths and returns a summary.
func (a *App) Clean() cleanResult {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return cleanResult{Message: "正在扫描或清理"}
	}
	if a.result == nil {
		a.mu.Unlock()
		return cleanResult{Empty: true, Message: "请先扫描"}
	}
	jobs := scan.JobsFrom(a.result.Items)
	roots := append([]string(nil), a.result.Roots...)
	if len(jobs) == 0 {
		a.mu.Unlock()
		return cleanResult{Empty: true, Message: "请先勾选要清理的项目"}
	}
	a.busy = true
	a.mu.Unlock()
	defer a.end()

	a.mu.Lock()
	a.status = fmt.Sprintf("正在清理 0 / %d", len(jobs))
	a.done, a.total = 0, len(jobs)
	a.mu.Unlock()
	a.publish()

	stats := scan.Clean(jobs, roots, a.appendLog, func(done, total int) {
		a.mu.Lock()
		a.done, a.total = done, total
		a.status = fmt.Sprintf("正在清理 %d / %d", done, total)
		a.mu.Unlock()
		a.touch()
	})
	msg := fmt.Sprintf("已删除 %d 项，失败 %d 项，约释放 %s。", stats.Deleted, stats.Failed, scan.FormatSize(stats.Freed))
	if len(stats.Errors) > 0 {
		msg += "\n\n" + strings.Join(stats.Errors, "\n")
	}
	a.mu.Lock()
	a.done, a.total = 0, 0
	a.mu.Unlock()
	return cleanResult{
		Deleted:   stats.Deleted,
		Failed:    stats.Failed,
		FreedText: scan.FormatSize(stats.Freed),
		Errors:    stats.Errors,
		Message:   msg,
	}
}

// begin marks a scan as running.
func (a *App) begin() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return false
	}
	a.busy = true
	return true
}

// end allows another scan and refreshes the window.
func (a *App) end() {
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
	a.publish()
}

// setAll applies one checkbox value to every discovered item.
func (a *App) setAll(pick func(*scan.Item) bool) {
	a.mu.Lock()
	if a.busy || a.result == nil {
		a.mu.Unlock()
		return
	}
	for _, it := range a.result.Items {
		it.Checked = pick(it)
	}
	a.refreshStatusLocked()
	a.mu.Unlock()
	a.publish()
}

// applyResultLocked installs a new scan. The caller holds a.mu.
func (a *App) applyResultLocked(result *scan.Result) {
	a.result = result
	a.selectedID = ""
	if result == nil || len(result.Groups) == 0 {
		return
	}
	a.selectedID = result.Groups[0].Class.ID
}

// sortGroupsLocked orders checked groups first and larger groups earlier.
// The caller holds a.mu.
func (a *App) sortGroupsLocked() {
	if a.result == nil {
		return
	}
	id := a.selectedID
	groups := a.result.Groups
	sort.SliceStable(groups, func(i, j int) bool {
		iOn := groups[i].SelectedCount() > 0
		jOn := groups[j].SelectedCount() > 0
		if iOn != jOn {
			return iOn
		}
		return groups[i].TotalSize() > groups[j].TotalSize()
	})
	if a.findGroupLocked(id) != nil {
		return
	}
	a.selectedID = ""
	if len(groups) == 0 {
		return
	}
	a.selectedID = groups[0].Class.ID
}

// refreshStatusLocked writes the idle summary. The caller holds a.mu.
func (a *App) refreshStatusLocked() {
	if a.result == nil {
		a.status = "准备扫描…"
		return
	}
	chosen := 0
	for _, it := range a.result.Items {
		if it.Checked {
			chosen++
		}
	}
	a.status = fmt.Sprintf("共 %d 个分组、%d 项，已选 %d 项", len(a.result.Groups), len(a.result.Items), chosen)
}

// findGroupLocked returns the group with id. The caller holds a.mu.
func (a *App) findGroupLocked(id string) *scan.Group {
	if a.result == nil || id == "" {
		return nil
	}
	for _, g := range a.result.Groups {
		if g.Class.ID == id {
			return g
		}
	}
	return nil
}

// snapshot copies the lists for one event.
func (a *App) snapshot() viewState {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := viewState{
		Busy:       a.busy,
		Status:     a.status,
		SelectedID: a.selectedID,
		Done:       a.done,
		Total:      a.total,
		Groups:     []groupView{},
		Items:      []itemView{},
		Logs:       append([]string(nil), a.logs...),
		ChosenText: scan.FormatSize(0),
	}
	if a.result == nil {
		return st
	}
	var bytes int64
	for _, it := range a.result.Items {
		if !it.Checked {
			continue
		}
		st.Chosen++
		bytes += it.Size()
	}
	st.ChosenText = scan.FormatSize(bytes)
	st.Groups = make([]groupView, 0, len(a.result.Groups))
	for _, g := range a.result.Groups {
		st.Groups = append(st.Groups, groupView{
			ID:       g.Class.ID,
			Title:    g.Class.Title,
			Reason:   g.Class.Reason,
			Count:    len(g.Items),
			Selected: g.SelectedCount(),
			SizeText: scan.FormatSize(g.TotalSize()),
			Checked:  g.AllChecked(),
		})
	}
	g := a.findGroupLocked(a.selectedID)
	if g == nil {
		return st
	}
	st.Items = make([]itemView, 0, len(g.Items))
	for i, it := range g.Items {
		st.Items = append(st.Items, itemView{
			Index:    i,
			Name:     it.Name,
			Path:     it.Path,
			Kind:     itemKind(it),
			SizeText: scan.FormatSize(it.Size()),
			Checked:  it.Checked,
		})
	}
	return st
}

// publish sends the current snapshot to the window.
func (a *App) publish() {
	a.mu.Lock()
	ctx := a.ctx
	a.lastEmit = time.Now()
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "state", a.snapshot())
}

// touch publishes at most twice a second while a scan is measuring.
func (a *App) touch() {
	now := time.Now()
	a.mu.Lock()
	if now.Sub(a.lastEmit) < 500*time.Millisecond {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.publish()
}

// appendLog keeps the latest English diagnostics and prints them.
func (a *App) appendLog(msg string) {
	log.Printf("%s", msg)
	a.mu.Lock()
	a.logs = append(a.logs, msg)
	if len(a.logs) > 200 {
		a.logs = append([]string(nil), a.logs[len(a.logs)-200:]...)
	}
	a.mu.Unlock()
}

// itemKind is the short type label shown on a file row.
func itemKind(it *scan.Item) string {
	if it.Reparse {
		return "链接"
	}
	if it.IsDir {
		return "目录"
	}
	return "文件"
}
