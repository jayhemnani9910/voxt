package hotkey

import "testing"

func TestNewHotkeyListener(t *testing.T) {
	noop := func() {}

	if l, err := NewHotkeyListener("f9", noop); err != nil || l.keyCode != 67 {
		t.Errorf("f9: listener %+v, err %v", l, err)
	}
	if _, err := NewHotkeyListener("F13", noop); err == nil {
		t.Error("F13: expected an error")
	}
	if _, err := NewHotkeyListener("F9", nil); err == nil {
		t.Error("nil callback: expected an error")
	}
}
