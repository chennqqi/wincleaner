package scan

import "testing"

// TestClassifyMergesCompilerTemps checks Go build, gopls, and go mod names.
func TestClassifyMergesCompilerTemps(t *testing.T) {
	build := []string{"go-build12345", "aurivex-core-review-go-build", "aurivex-admission-gocache", "proj-go-cache"}
	for _, name := range build {
		got := Classify(name)
		if got.ID != "go-build" || !got.DefaultChecked(1) {
			t.Fatalf("%s -> %+v", name, got)
		}
	}
	if Classify("gopls-11176.1").ID != "gopls" || Classify("gopls-12124.1").ID != "gopls" {
		t.Fatal("gopls names should share a group")
	}
	if Classify("go-mod").ID != "go-mod" || Classify("go-mod-tmp991").ID != "go-mod" {
		t.Fatal("go mod names should share a group")
	}
	if Classify("mygo-build").ID == "go-build" {
		t.Fatal("mygo-build is not a go-build directory")
	}
}

// TestClassifyGeneratedPattern checks that random suffixes collapse and fixed words do not.
func TestClassifyGeneratedPattern(t *testing.T) {
	a := Classify("skillaudit-ast-python-12")
	b := Classify("skillaudit-ast-python-99")
	c := Classify("skillaudit-ast-clean-json-3")
	if a.ID != b.ID {
		t.Fatalf("python temps should merge: %s vs %s", a.ID, b.ID)
	}
	if a.ID == c.ID {
		t.Fatalf("clean-json is a different pattern: %s", c.ID)
	}
	if a.Title != "skillaudit-ast-python-*" {
		t.Fatalf("title=%s", a.Title)
	}
	if Classify("codebuddy-marketplace-update-lBD3nW").ID != "editor" {
		t.Fatal("editor temps should merge")
	}
	mixed := Classify("cache-worker-lBD3nW")
	if mixed.Title != "cache-worker-*" {
		t.Fatal(mixed.Title)
	}
	if Classify("playwright").Policy != PolicyUncheck {
		t.Fatal("a single unrecognized name stays unchecked")
	}
}

// TestClassifyDefaults checks which names start selected.
func TestClassifyDefaults(t *testing.T) {
	if !Classify("is-08TEB.tmp").DefaultChecked(1) {
		t.Fatal("tmp files are selected")
	}
	if Classify("notes.json").DefaultChecked(3) {
		t.Fatal("json stays unselected")
	}
	if Classify("ScreenShot_1.png").DefaultChecked(30) {
		t.Fatal("images stay unselected")
	}
	if !Classify("{003DDA2F-047F-4985-8169-384E3743AEE5}").DefaultChecked(1) {
		t.Fatal("guid directories are selected")
	}
	if Classify("004e2df9-16c5-415a-8672-55c3b5756c1e").ID != "guid" {
		t.Fatal("uuid directories are guid temps")
	}
	pat := Classify("foo-1")
	if pat.DefaultChecked(1) || !pat.DefaultChecked(2) {
		t.Fatal("a generated pattern is selected only when repeated")
	}
	if Classify("chromecrx_chrome_Unpacker_BeginUnzipping17588_516813440").ID != "browser" {
		t.Fatal("chrome unpack dirs should merge")
	}
	if Classify("npm").ID != "node" || Classify("yarn--1762771420106-0.2").ID != "node" {
		t.Fatal("node toolchain should merge")
	}
}
