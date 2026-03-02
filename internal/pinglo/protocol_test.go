package pinglo

import (
	"os"
	"testing"
)

func TestManagerStartFinishAndClear(t *testing.T) {
	changes := 0
	mgr := NewManager(func() {
		changes++
	})

	item1 := mgr.Start("/tmp", "build")
	if item1.Status != StatusRunning {
		t.Fatalf("expected running, got %s", item1.Status)
	}
	_ = mgr.Start("/tmp", "test")
	if len(mgr.List()) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mgr.List()))
	}

	// finish first command
	mgr.Finish("/tmp", "build", 0)
	items := mgr.List()
	if items[0].Status != StatusSuccess {
		t.Fatalf("expected success after finish, got %s", items[0].Status)
	}

	// finish second command with failure
	mgr.Finish("/tmp", "test", 1)
	items = mgr.List()
	if items[1].Status != StatusFailed {
		t.Fatalf("expected failed, got %s", items[1].Status)
	}

	if changes < 4 {
		t.Fatalf("expected at least 4 change notifications, got %d", changes)
	}

	mgr.Clear()
	if len(mgr.List()) != 0 {
		t.Fatalf("expected empty list after clear")
	}
}

func TestManagerDeduplication(t *testing.T) {
	mgr := NewManager(nil)
	mgr.Start("/tmp/project", "sync")
	mgr.Start("/tmp/project", "sync")
	if len(mgr.List()) != 1 {
		t.Fatalf("duplicate start should keep single entry")
	}

	mgr.Finish("/tmp/project", "sync", 0)
	if len(mgr.List()) != 1 {
		t.Fatalf("finish should not create extra entries")
	}
}

func TestDefaultSocketPathEnvPrecedence(t *testing.T) {
	prev := os.Getenv("PINGLO_SOCKET")
	defer os.Setenv("PINGLO_SOCKET", prev)

	os.Setenv("PINGLO_SOCKET", "/tmp/custom.sock")
	if got := DefaultSocketPath(); got != "/tmp/custom.sock" {
		t.Fatalf("expected custom socket, got %s", got)
	}
	os.Unsetenv("PINGLO_SOCKET")

	prevRuntime := os.Getenv("XDG_RUNTIME_DIR")
	defer os.Setenv("XDG_RUNTIME_DIR", prevRuntime)

	os.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime")
	expected := "/tmp/runtime/pinglo.sock"
	if got := DefaultSocketPath(); got != expected {
		t.Fatalf("expected runtime socket, got %s", got)
	}
	os.Unsetenv("XDG_RUNTIME_DIR")

	if got := DefaultSocketPath(); got == "" {
		t.Fatalf("expected fallback socket, got empty")
	}
}

func TestSetDotCustom(t *testing.T) {
	mgr := NewManager(nil)
	mgr.SetDot("custom-1", "#123456", "tooltip text", StatusRunning)
	items := mgr.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 dot, got %d", len(items))
	}
	if items[0].Color != "#123456" || items[0].Tooltip != "tooltip text" || items[0].ID != "custom-1" {
		t.Fatalf("dot did not keep custom attributes: %+v", items[0])
	}
}

func TestRemoveDot(t *testing.T) {
	mgr := NewManager(nil)
	mgr.SetDot("a", "", "", StatusFailed)
	if !mgr.RemoveDot("a") {
		t.Fatalf("expected remove to succeed")
	}
	if len(mgr.List()) != 0 {
		t.Fatalf("expected zero items after remove")
	}
	if mgr.RemoveDot("a") {
		t.Fatalf("removing absent dot should return false")
	}
}
