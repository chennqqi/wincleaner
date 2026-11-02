// Package scan finds temporary files, groups paths that share a naming
// pattern, and deletes only the items the user selected.
package scan

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Policy decides the default checkbox for a classified name.
type Policy int

const (
	// PolicyUncheck keeps the item visible but not selected.
	PolicyUncheck Policy = iota
	// PolicyCheck selects regenerable temporary data.
	PolicyCheck
	// PolicyByCount selects a generated pattern when at least two names share it.
	PolicyByCount
)

// Class is the merged group a temp name belongs to.
type Class struct {
	ID     string
	Title  string
	Reason string
	Policy Policy
	// Leaf means the directory is one deletion unit and is not opened.
	Leaf bool
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Classify maps a file or directory name to a cleanup group.
func Classify(name string) Class {
	low := strings.ToLower(name)
	if class, ok := knownClass(low); ok {
		return class
	}
	if ext := strings.ToLower(filepath.Ext(name)); ext != "" {
		if class, ok := extClass(ext); ok {
			return class
		}
	}
	if isGUID(name) {
		return Class{
			ID:     "guid",
			Title:  "GUID 临时目录",
			Reason: "以 GUID 命名的临时目录，通常由安装或组件释放",
			Policy: PolicyCheck,
		}
	}
	pattern, generated := normalizePattern(name)
	if !generated {
		return Class{
			ID:     "one:" + low,
			Title:  name,
			Reason: "未识别的单个项目，默认不清理",
			Policy: PolicyUncheck,
		}
	}
	title := pattern
	return Class{
		ID:     "pat:" + strings.ToLower(title),
		Title:  title,
		Reason: "同一命名模式的临时文件，出现多次时默认清理",
		Policy: PolicyByCount,
	}
}

// DefaultChecked reports whether count items of this class start selected.
func (c Class) DefaultChecked(count int) bool {
	switch c.Policy {
	case PolicyCheck:
		return true
	case PolicyByCount:
		return count >= 2
	default:
		return false
	}
}

// knownClass matches compiler and tool directories that should be merged by family.
func knownClass(low string) (Class, bool) {
	switch {
	case isGopls(low):
		return Class{ID: "gopls", Title: "Gopls 临时目录", Reason: "gopls 语言服务临时目录，删除后会重建", Policy: PolicyCheck, Leaf: true}, true
	case isGoBuild(low):
		return Class{ID: "go-build", Title: "Go build 临时目录", Reason: "Go 编译缓存和 go-build 临时目录，下次编译会重建", Policy: PolicyCheck, Leaf: true}, true
	case isGoMod(low):
		return Class{ID: "go-mod", Title: "Go module 临时目录", Reason: "Go 模块临时目录，删除后需要重新下载", Policy: PolicyCheck, Leaf: true}, true
	case isNode(low):
		return Class{ID: "node", Title: "Node.js 工具链临时目录", Reason: "npm、yarn、esbuild 等临时目录", Policy: PolicyCheck, Leaf: true}, true
	case isPython(low):
		return Class{ID: "python", Title: "Python 临时目录", Reason: "pip 等 Python 临时目录", Policy: PolicyCheck, Leaf: true}, true
	case isRust(low):
		return Class{ID: "rust", Title: "Rust 临时目录", Reason: "rustc / cargo 临时目录", Policy: PolicyCheck, Leaf: true}, true
	case strings.Contains(low, "jetbrains"):
		return Class{ID: "jetbrains", Title: "JetBrains 临时目录", Reason: "IDE 临时文件", Policy: PolicyCheck, Leaf: true}, true
	case isBrowser(low):
		return Class{ID: "browser", Title: "浏览器临时目录", Reason: "浏览器解压或缓存产生的临时目录", Policy: PolicyCheck, Leaf: true}, true
	case isEditor(low):
		return Class{ID: "editor", Title: "编辑器临时目录", Reason: "编辑器临时目录；正在占用的文件会跳过", Policy: PolicyCheck, Leaf: true}, true
	case strings.HasPrefix(low, ".tmp") || strings.Contains(low, "msbuild"):
		return Class{ID: "msbuild", Title: "MSBuild 临时文件", Reason: "编译器或安装器留下的临时文件", Policy: PolicyCheck, Leaf: true}, true
	default:
		return Class{}, false
	}
}

// extClass groups files that share a cleanup policy by extension.
func extClass(ext string) (Class, bool) {
	switch ext {
	case ".dmp", ".mdmp", ".hdmp":
		return Class{ID: "dump", Title: "转储文件 (*" + ext + ")", Reason: "崩溃转储，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".mp4", ".mov", ".wav", ".mp3":
		return Class{ID: "media:" + ext, Title: "媒体文件 (*" + ext + ")", Reason: "图片、图标或媒体，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".zip", ".7z", ".rar", ".iso":
		return Class{ID: "archive:" + ext, Title: "压缩包 (*" + ext + ")", Reason: "压缩包可能仍有用，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".exe", ".dll", ".sys", ".msi", ".msp":
		return Class{ID: "binary:" + ext, Title: "程序文件 (*" + ext + ")", Reason: "程序或安装包，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".sql", ".env":
		return Class{ID: "config:" + ext, Title: "配置或数据 (*" + ext + ")", Reason: "配置或数据文件，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".go", ".py", ".js", ".ts", ".c", ".h", ".cs", ".java", ".ps1", ".sh", ".bat", ".cmd":
		return Class{ID: "source:" + ext, Title: "源码文件 (*" + ext + ")", Reason: "源码文件，默认保留", Policy: PolicyUncheck, Leaf: true}, true
	case ".tmp", ".temp":
		return Class{ID: "tmp", Title: "临时文件 (*.tmp)", Reason: "扩展名为临时文件，默认可清理", Policy: PolicyCheck, Leaf: true}, true
	case ".log", ".etl":
		return Class{ID: "log", Title: "日志文件 (*" + ext + ")", Reason: "临时日志", Policy: PolicyCheck, Leaf: true}, true
	default:
		return Class{}, false
	}
}

// isGopls reports a gopls temporary directory name.
func isGopls(low string) bool {
	return strings.HasPrefix(low, "gopls")
}

// isGoBuild reports a Go build cache or go-build work directory.
func isGoBuild(low string) bool {
	if low == "go-build" || strings.HasPrefix(low, "go-build") {
		return true
	}
	if strings.HasSuffix(low, "-go-build") || strings.HasSuffix(low, "_go-build") {
		return true
	}
	return hasWord(low, "gocache") || hasWord(low, "go-cache")
}

// isGoMod reports a Go module temporary directory.
func isGoMod(low string) bool {
	if strings.HasPrefix(low, "go-mod") {
		return true
	}
	return hasWord(low, "gomodcache") || hasWord(low, "gomod")
}

// isNode reports an npm, yarn, or esbuild temporary directory.
func isNode(low string) bool {
	switch low {
	case "npm", "node-compile-cache", "node-jiti", "node-gyp":
		return true
	}
	return strings.HasPrefix(low, "npm-") || strings.HasPrefix(low, "npx-") || strings.HasPrefix(low, "yarn-") || strings.HasPrefix(low, "esbuild-")
}

// isPython reports a pip or pytest temporary directory.
func isPython(low string) bool {
	return strings.HasPrefix(low, "pip-") || strings.HasPrefix(low, "pytest-") || strings.HasPrefix(low, "pyright")
}

// isRust reports a rustc or cargo temporary directory.
func isRust(low string) bool {
	return strings.HasPrefix(low, "rustc") || strings.HasPrefix(low, "cargo-") || low == "cargo"
}

// isBrowser reports a browser unpack or cache directory in Temp.
func isBrowser(low string) bool {
	return strings.HasPrefix(low, "chromecrx") || strings.HasPrefix(low, "scoped_dir") || strings.HasPrefix(low, "msedge") || strings.HasPrefix(low, ".com.google.chrome")
}

// isEditor reports an editor temporary directory such as vscode or codex.
func isEditor(low string) bool {
	return strings.HasPrefix(low, "vscode-") || strings.HasPrefix(low, "vscode") || strings.HasPrefix(low, "codebuddy-") || strings.HasPrefix(low, "codex-")
}

// isGUID reports a GUID or content-hash directory name.
func isGUID(name string) bool {
	s := strings.TrimPrefix(name, "{")
	s = strings.TrimSuffix(s, "}")
	if uuidPattern.MatchString(s) {
		return true
	}
	if len(s) == 32 || len(s) == 40 || len(s) == 64 {
		return isHex(s)
	}
	return false
}

// hasWord reports whether word appears as its own token in name.
func hasWord(name, word string) bool {
	rest := name
	for {
		i := strings.Index(rest, word)
		if i < 0 {
			return false
		}
		end := i + len(word)
		leftOK := i == 0 || !isNameChar(rest[i-1])
		rightOK := end == len(rest) || !isNameChar(rest[end])
		if leftOK && rightOK {
			return true
		}
		rest = rest[i+1:]
	}
}

// isNameChar reports a letter or digit used as a token boundary.
func isNameChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// normalizePattern replaces random suffixes so sibling temp dirs share a key.
func normalizePattern(name string) (string, bool) {
	ext := ""
	base := name
	if e := filepath.Ext(name); keptExt(e) {
		ext = strings.ToLower(e)
		base = name[:len(name)-len(e)]
	}
	var b strings.Builder
	changed := false
	start := 0
	writeTok := func(tok, sep string) {
		if tok != "" && randomToken(tok) {
			b.WriteByte('*')
			changed = true
		} else {
			b.WriteString(tok)
		}
		b.WriteString(sep)
	}
	for i := 0; i < len(base); i++ {
		if base[i] != '-' && base[i] != '_' {
			continue
		}
		writeTok(base[start:i], base[i:i+1])
		start = i + 1
	}
	writeTok(base[start:], "")
	out := collapseStars(b.String()) + ext
	if out == "" {
		return "*", true
	}
	return out, changed
}

// keptExt reports extensions that stay visible in a merged pattern.
func keptExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".tmp", ".temp", ".log", ".txt", ".png", ".jpg", ".zip", ".json":
		return true
	default:
		return false
	}
}

// randomToken reports a temp suffix generated by a tool rather than a fixed word.
func randomToken(tok string) bool {
	if tok == "" || tok == "*" {
		return false
	}
	if hasDigit(tok) {
		return true
	}
	if len(tok) >= 16 && isHex(tok) {
		return true
	}
	if len(tok) >= 4 && hasASCIILetter(tok, false) && hasASCIILetter(tok, true) {
		return true
	}
	return false
}

// hasDigit reports whether s contains an ASCII digit.
func hasDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			return true
		}
	}
	return false
}

// hasASCIILetter reports an ASCII letter of the requested case.
func hasASCIILetter(s string, upper bool) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if upper && c >= 'A' && c <= 'Z' {
			return true
		}
		if !upper && c >= 'a' && c <= 'z' {
			return true
		}
	}
	return false
}

// isHex reports a hexadecimal string.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// collapseStars turns adjacent wildcards into one.
func collapseStars(s string) string {
	for strings.Contains(s, "**") {
		s = strings.ReplaceAll(s, "**", "*")
	}
	return s
}

// bulkPrefix reports names that are already a repeated leaf pattern.
// Their contents are not listed one by one.
func bulkPrefix(name string) bool {
	low := strings.ToLower(name)
	prefixes := []string{"skillaudit-", "chromecrx", "scoped_dir"}
	for _, p := range prefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// isContainer reports a directory whose children should be grouped on their own.
func isContainer(name string) bool {
	low := strings.ToLower(name)
	switch low {
	case "cursor-sandbox-cache", "sandbox-cache", "tmp", "temp", "cache":
		return true
	default:
		return strings.HasSuffix(low, "-sandbox-cache")
	}
}

// isHexName reports a hash bucket directory.
func isHexName(name string) bool {
	return len(name) >= 16 && isHex(name)
}

// compilerChild reports a directory that should be lifted into a Go group.
func compilerChild(name string) bool {
	switch Classify(name).ID {
	case "go-build", "gopls", "go-mod":
		return true
	default:
		return false
	}
}
