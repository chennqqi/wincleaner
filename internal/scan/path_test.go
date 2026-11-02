package scan

import "testing"

// TestWithinRoots checks that only children of a temp root can be deleted.
func TestWithinRoots(t *testing.T) {
	root := `C:\Users\worker\AppData\Local\Temp`
	if !WithinRoots(root+`\gopls-1`, []string{root}) {
		t.Fatal("child should be inside the temp root")
	}
	if !WithinRoots(`c:\users\worker\appdata\local\temp\a`, []string{root}) {
		t.Fatal("windows paths are case-insensitive")
	}
	if WithinRoots(root, []string{root}) {
		t.Fatal("the temp root itself is not deletable")
	}
	if WithinRoots(`C:\Windows\System32\cmd.exe`, []string{root}) {
		t.Fatal("system32 is outside temp")
	}
	if WithinRoots(`D:\Temp\x`, []string{root}) {
		t.Fatal("another drive is outside temp")
	}
	if WithinRoots(`C:\TempExtra\a`, []string{`C:\Temp`}) {
		t.Fatal("a longer sibling prefix is not inside the root")
	}
}

// TestFormatSize checks byte and kilobyte rendering.
func TestFormatSize(t *testing.T) {
	if FormatSize(0) != "0 B" {
		t.Fatal(FormatSize(0))
	}
	if FormatSize(1536) != "1.5 KB" {
		t.Fatal(FormatSize(1536))
	}
}
