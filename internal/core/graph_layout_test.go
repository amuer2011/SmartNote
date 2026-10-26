package core

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGraphLayoutPersistenceIsolationAndBackup(t *testing.T) {
	s, w := fixture(t)
	_ = mustNote(t, s, "图谱")
	l := GraphLayout{Positions: map[string]GraphPoint{"node": {X: 123, Y: -456}}, Zoom: 1.6, Pan: GraphPoint{X: 50, Y: 80}}
	key := "global"
	if e := s.SaveGraphLayout(w.ID, key, l); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveGraphLayout(w.ID, "local", GraphLayout{Positions: l.Positions, Zoom: .5, Pan: l.Pan}); e != nil {
		t.Fatal(e)
	}
	other, e := s.AddWorkspace("隔离", filepath.Join(t.TempDir(), "other"))
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.GetGraphLayouts(other.ID)
	if e != nil || len(got) != 0 {
		t.Fatal("workspace layout leaked", e)
	}
	got, e = s.GetGraphLayouts(w.ID)
	if e != nil || !reflect.DeepEqual(got[key], l) || len(got) != 2 {
		t.Fatalf("%+v %v", got, e)
	}
	// Pending saves for the workspace just left must still target its own data.
	if e = s.SaveGraphLayout(w.ID, key, l); e != nil {
		t.Fatal(e)
	}
	if e = s.OpenWorkspace(w.ID); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "native.zip")
	if e = s.Export(archive, true); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Import(archive, filepath.Join(t.TempDir(), "restored"))
	if e != nil {
		t.Fatal(e)
	}
	got, e = s.GetGraphLayouts(restored.ID)
	if e != nil || !reflect.DeepEqual(got[key], l) {
		t.Fatal("backup lost graph layout", e)
	}
	// Read from a fresh service to check disk persistence, not an in-memory cache.
	fresh, e := New(s.root)
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	got, e = fresh.GetGraphLayouts(w.ID)
	if e != nil || !reflect.DeepEqual(got[key], l) {
		t.Fatal("restart lost graph layout", e)
	}
}

func TestGraphLayoutRejectsInvalidStateWithoutReplacing(t *testing.T) {
	s, w := fixture(t)
	l := GraphLayout{Positions: map[string]GraphPoint{"n": {X: 1, Y: 2}}, Zoom: 1}
	if e := s.SaveGraphLayout(w.ID, "global", l); e != nil {
		t.Fatal(e)
	}
	for _, v := range []float64{0, 4, math.NaN(), math.Inf(1)} {
		bad := l
		bad.Zoom = v
		if e := s.SaveGraphLayout(w.ID, "global", bad); e == nil {
			t.Fatal("invalid zoom accepted")
		}
	}
	bad := l
	bad.Pan.X = math.Inf(1)
	if e := s.SaveGraphLayout(w.ID, "global", bad); e == nil {
		t.Fatal("invalid pan accepted")
	}
	if e := s.SaveGraphLayout("missing", "global", l); e == nil {
		t.Fatal("unknown workspace accepted")
	}
	got, e := s.GetGraphLayouts(w.ID)
	if e != nil || !reflect.DeepEqual(got["global"], l) {
		t.Fatal("rejected save damaged previous layout", e)
	}
}
