package core

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestUIPreferencesPersistAndWorkspaceListingDoesNotSwitch(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.AddWorkspace("first", filepath.Join(root, "first"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.CreateNote("First", ".")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AddWorkspace("second", filepath.Join(root, "second"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := s.ListWorkspaceFiles(first.ID)
	if err != nil || len(files.Notes) != 1 || files.Notes[0].ID != n.ID {
		t.Fatalf("inactive workspace: %+v %v", files, err)
	}
	if s.Settings().ActiveID != second.ID {
		t.Fatal("listing switched active workspace")
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14}); err != nil || s.Settings().UI.FontFamily != "system" {
		t.Fatalf("legacy font default: %v", err)
	}
	ui := UIPreferences{FontSize: 18, FontFamily: "FangSong", CollapsedWorkspaces: []string{second.ID}, EntryOrder: map[string]map[string][]string{first.ID: {"": {n.Path, "folder"}}}}
	if err := s.SetUIPreferences(ui); err != nil {
		t.Fatal(err)
	}
	bad := ui
	bad.FontSize = 100
	if err := s.SetUIPreferences(bad); err == nil {
		t.Fatal("invalid font size accepted")
	}
	bad = ui
	bad.FontFamily = "unrecognized-font"
	if err := s.SetUIPreferences(bad); err == nil {
		t.Fatal("unknown font accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got := s.Settings().UI
	if got.FontSize != 18 || got.FontFamily != "FangSong" || len(got.CollapsedWorkspaces) != 1 || got.EntryOrder[first.ID][""][0] != n.Path {
		t.Fatalf("preferences lost: %+v", got)
	}
}

// supportedFontFamilies mirrors the allow-list enforced by SetUIPreferences and
// the option list rendered by frontend/src/services/fonts.ts.
var supportedFontFamilies = []string{
	"system", "SimSun", "NSimSun", "FangSong", "SimHei", "KaiTi",
	"Microsoft YaHei", "DengXian", "STSong", "STFangsong", "STKaiti",
	"STXihei", "STZhongsong", "FZShuTi", "FZYaoti", "Source Han Sans SC",
	"Source Han Serif SC", "Noto Sans CJK SC", "Noto Serif CJK SC",
	"Arial", "Times New Roman", "Consolas",
}

func TestUIPreferencesFontSizeBoundaries(t *testing.T) {
	s, _ := fixture(t)
	for _, size := range []int{12, 13, 18, 22} {
		if err := s.SetUIPreferences(UIPreferences{FontSize: size}); err != nil {
			t.Fatalf("font size %d rejected: %v", size, err)
		}
		got := s.Settings().UI
		if got.FontSize != size {
			t.Fatalf("font size %d stored as %d", size, got.FontSize)
		}
		if got.FontFamily != "system" {
			t.Fatalf("font size %d changed the default family to %q", size, got.FontFamily)
		}
	}
	// The last accepted size is 22; rejected attempts must not mutate stored state.
	for _, size := range []int{11, 0, -1, 23, 100} {
		if err := s.SetUIPreferences(UIPreferences{FontSize: size}); err == nil {
			t.Fatalf("font size %d accepted", size)
		}
		if got := s.Settings().UI.FontSize; got != 22 {
			t.Fatalf("rejected font size %d changed stored size to %d", size, got)
		}
	}
}

func TestUIPreferencesFontFamilies(t *testing.T) {
	s, _ := fixture(t)
	for _, family := range supportedFontFamilies {
		if err := s.SetUIPreferences(UIPreferences{FontSize: 14, FontFamily: family}); err != nil {
			t.Fatalf("supported font %q rejected: %v", family, err)
		}
		if got := s.Settings().UI.FontFamily; got != family {
			t.Fatalf("font %q stored as %q", family, got)
		}
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14, FontFamily: ""}); err != nil {
		t.Fatalf("empty font family rejected: %v", err)
	}
	if got := s.Settings().UI.FontFamily; got != "system" {
		t.Fatalf("empty font family did not default to system: %q", got)
	}
	// Matching is exact and case-sensitive; display names are not identifiers.
	for _, family := range []string{"fangsong", "FangSong ", " FangSong", "system\n", "宋体", "微软雅黑", "Comic Sans MS"} {
		if err := s.SetUIPreferences(UIPreferences{FontSize: 14, FontFamily: family}); err == nil {
			t.Fatalf("unsupported font %q accepted", family)
		}
		if got := s.Settings().UI.FontFamily; got != "system" {
			t.Fatalf("rejected font %q changed stored family to %q", family, got)
		}
	}
}

func TestUIPreferencesDefaultFontFamilyPersists(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.Settings().UI; got.FontFamily != "system" || got.FontSize != 14 {
		t.Fatalf("default font family not persisted: %+v", got)
	}
}

func TestUIPreferencesRejectedUpdateKeepsStoredState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.AddWorkspace("first", filepath.Join(root, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AddWorkspace("second", filepath.Join(root, "second"))
	if err != nil {
		t.Fatal(err)
	}
	want := UIPreferences{
		Layout:           LayoutPreferences{SidebarWidth: 244, ListWidth: 292, InspectorWidth: 206, SplitRatio: 50},
		EditorFontFamily: "KaiTi", EditorFontSize: 14,
		FontSize:            16,
		FontFamily:          "KaiTi",
		CollapsedWorkspaces: []string{first.ID, second.ID},
		EntryOrder: map[string]map[string][]string{
			first.ID:  {"": {"b.md", "a.md"}, "folder": {"x.md"}},
			second.ID: {"": {"only.md"}},
		},
	}
	if err := s.SetUIPreferences(want); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]UIPreferences{
		"font size too large": {FontSize: 23, FontFamily: "KaiTi"},
		"font size too small": {FontSize: 11, FontFamily: "KaiTi"},
		"unknown font":        {FontSize: 16, FontFamily: "Papyrus"},
	} {
		if err := s.SetUIPreferences(bad); err == nil {
			t.Fatalf("%s: invalid preferences accepted", name)
		}
		if got := s.Settings().UI; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: rejected update mutated stored preferences: %+v", name, got)
		}
	}
	// The active workspace is unrelated to UI preferences and must survive too.
	if s.Settings().ActiveID != second.ID {
		t.Fatalf("active workspace changed in memory: %s", s.Settings().ActiveID)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.Settings().UI; !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences not persisted: got %+v want %+v", got, want)
	}
	if s.Settings().ActiveID != second.ID {
		t.Fatalf("active workspace not persisted: %s", s.Settings().ActiveID)
	}
}

func TestListWorkspaceFilesInactiveUnknownAndDirectories(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.AddWorkspace("first", filepath.Join(root, "first"))
	if err != nil {
		t.Fatal(err)
	}
	note, err := s.CreateNote("First", ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"空目录/子目录", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(first.Path, filepath.FromSlash(dir)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	second, err := s.AddWorkspace("second", filepath.Join(root, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateNote("Second", "."); err != nil {
		t.Fatal(err)
	}

	files, err := s.ListWorkspaceFiles(first.ID)
	if err != nil || len(files.Notes) != 1 || files.Notes[0].ID != note.ID {
		t.Fatalf("inactive workspace listing: %+v %v", files, err)
	}
	for _, dir := range []string{"空目录", "空目录/子目录"} {
		if !slices.Contains(files.Directories, dir) {
			t.Fatalf("missing directory %q in %v", dir, files.Directories)
		}
	}
	if slices.Contains(files.Directories, ".hidden") {
		t.Fatal("hidden directory exposed by listing")
	}
	if s.Settings().ActiveID != second.ID {
		t.Fatalf("listing switched active workspace to %s", s.Settings().ActiveID)
	}
	if _, err := s.ListWorkspaceFiles("does-not-exist"); err == nil {
		t.Fatal("unknown workspace listing accepted")
	}
	if s.Settings().ActiveID != second.ID {
		t.Fatalf("failed listing switched active workspace to %s", s.Settings().ActiveID)
	}
	// Read-only listing must also leave the persisted active workspace untouched.
	var persisted Settings
	if err := readJSON(filepath.Join(root, "data", "settings.json"), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ActiveID != second.ID {
		t.Fatalf("persisted active workspace changed to %s", persisted.ActiveID)
	}
}

func TestIndependentEditorTypography(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ui := UIPreferences{FontSize: 18, FontFamily: "FangSong", EditorFontSize: 24, EditorFontFamily: "SimHei", Shortcuts: map[string]string{"newFolder": "Ctrl+Alt+N", "rename": "Ctrl+Alt+M"}}
	if err := s.SetUIPreferences(ui); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []UIPreferences{
		{FontSize: 18, FontFamily: "FangSong", EditorFontSize: 49, EditorFontFamily: "SimHei"},
		{FontSize: 18, FontFamily: "FangSong", EditorFontSize: 9, EditorFontFamily: "SimHei"},
		{FontSize: 18, FontFamily: "FangSong", EditorFontSize: 24, EditorFontFamily: "unknown"},
	} {
		if s.SetUIPreferences(bad) == nil {
			t.Fatal("invalid editor typography accepted")
		}
	}
	s.Close()
	s, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got := s.Settings().UI
	if got.FontSize != 18 || got.FontFamily != "FangSong" || got.EditorFontSize != 24 || got.EditorFontFamily != "SimHei" || got.Shortcuts["newFolder"] != "Ctrl+Alt+N" || got.Shortcuts["rename"] != "Ctrl+Alt+M" {
		t.Fatalf("independent preferences lost: %+v", got)
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14, FontFamily: "KaiTi"}); err != nil {
		t.Fatal(err)
	}
	got = s.Settings().UI
	if got.EditorFontFamily != "KaiTi" || got.EditorFontSize != 14 {
		t.Fatalf("legacy migration: %+v", got)
	}
}

func TestLayoutPersistenceAndRejection(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ui := UIPreferences{FontSize: 14, Layout: LayoutPreferences{SidebarWidth: 280, ListWidth: 240, InspectorWidth: 180, SplitRatio: 60, HiddenPanels: []string{"preview", "inspector"}}}
	if err := s.SetUIPreferences(ui); err != nil {
		t.Fatal(err)
	}
	bad := ui
	bad.Layout.SplitRatio = 100
	if s.SetUIPreferences(bad) == nil {
		t.Fatal("invalid split accepted")
	}
	bad = ui
	bad.Layout.HiddenPanels = []string{"unknown"}
	if s.SetUIPreferences(bad) == nil {
		t.Fatal("invalid panel accepted")
	}
	s.Close()
	s, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !reflect.DeepEqual(s.Settings().UI.Layout, ui.Layout) {
		t.Fatalf("layout changed: %+v", s.Settings().UI.Layout)
	}
}
