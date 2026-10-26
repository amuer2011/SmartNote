package core

import (
	"reflect"
	"testing"
)

func TestRightPanelTabsPersistAndRejectInvalid(t *testing.T) {
	s, _ := fixture(t)
	ui := UIPreferences{FontSize: 14, Layout: LayoutPreferences{RightTabOrder: []string{"ai", "outline", "related"}, HiddenRightTabs: []string{"related", "outline"}}}
	if err := s.SetUIPreferences(ui); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	layout := fresh.Settings().UI.Layout
	if !reflect.DeepEqual(layout.RightTabOrder, ui.Layout.RightTabOrder) || !reflect.DeepEqual(layout.HiddenRightTabs, ui.Layout.HiddenRightTabs) {
		t.Fatal("right tabs not restored", layout)
	}
	for _, ids := range [][]string{{"unknown"}, {"ai", "ai"}} {
		bad := ui
		bad.Layout.RightTabOrder = ids
		if err := s.SetUIPreferences(bad); err == nil {
			t.Fatal("invalid tab order accepted", ids)
		}
	}
}
