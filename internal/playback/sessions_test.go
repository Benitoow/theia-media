package playback

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/Benitoow/theia-media/internal/fakeffmpeg"
)

// The registry's own rules, without a delivery involved: the remux ceiling,
// the transcode exemption, and the kill that finds every attached process.

func TestTheRemuxCeilingHoldsAndGivesSlotsBack(t *testing.T) {
	sessions := NewSessions(2, nil)

	first := sessions.Acquire(SessionRemux, "one")
	second := sessions.Acquire(SessionRemux, "two")
	if first == nil || second == nil {
		t.Fatal("the first two remuxes were refused their own ceiling")
	}
	if third := sessions.Acquire(SessionRemux, "three"); third != nil {
		t.Error("a third remux was admitted; the ceiling is two")
	}
	if sessions.RemuxActive() != 2 {
		t.Errorf("remux active = %d, want 2", sessions.RemuxActive())
	}

	second.Release()
	third := sessions.Acquire(SessionRemux, "three")
	if third == nil {
		t.Error("the ceiling did not give the released slot back")
	}
	first.Release()
	fourth := sessions.Acquire(SessionRemux, "four")
	if fourth == nil {
		t.Error("the second release did not free a slot")
	}
	third.Release()
	fourth.Release()
	if sessions.RemuxActive() != 0 || sessions.Active() != 0 {
		t.Errorf("after every release: active = %d remux = %d, want 0/0",
			sessions.Active(), sessions.RemuxActive())
	}
}

func TestTranscodesDoNotConsumeRemuxSlots(t *testing.T) {
	sessions := NewSessions(1, nil)

	if slot := sessions.Acquire(SessionRemux, "the only remux"); slot == nil {
		t.Fatal("the first remux was refused")
	}
	for i := 0; i < 3; i++ {
		if slot := sessions.Acquire(SessionTranscode, "transcode"); slot == nil {
			t.Errorf("transcode %d was refused; its budget is the transcode limiter, not this ceiling", i)
		}
	}
}

// KillAll walks every attached process, and nothing it has not been told
// about. The live children are this test binary again, sleeping in their fake
// role; a kill that failed would leave them sleeping until the suite's own
// timeout, which is exactly the orphan the registry exists to prevent.
func TestKillAllKillsEveryAttachedProcess(t *testing.T) {
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeLive, t.TempDir())
	defer restore()

	sessions := NewSessions(0, nil)
	slots := make([]*Slot, 3)
	for i := range slots {
		slots[i] = sessions.Acquire(SessionRemux, "sleeping")
		if slots[i] == nil {
			t.Fatal("the registry refused a slot under its default ceiling")
		}
		cmd := exec.Command(os.Args[0])
		cmd.Stdout = io.Discard
		if err := cmd.Start(); err != nil {
			t.Fatalf("starting the fake child: %v", err)
		}
		slots[i].Attach(cmd.Process)
	}
	if sessions.Active() != 3 {
		t.Fatalf("active = %d, want 3", sessions.Active())
	}

	if killed := sessions.KillAll(); killed != 3 {
		t.Errorf("KillAll killed %d, want 3", killed)
	}

	for _, slot := range slots {
		slot.Release()
	}
	if sessions.Active() != 0 {
		t.Errorf("active after the kill = %d, want 0", sessions.Active())
	}
}

func TestAProcessIsUnfindableOnceReleased(t *testing.T) {
	sessions := NewSessions(0, nil)
	slot := sessions.Acquire(SessionRemux, "gone")
	if slot == nil {
		t.Fatal("the first slot was refused")
	}
	slot.Release()
	slot.Release() // a double release is a no-op, not a negative count
	if sessions.RemuxActive() != 0 {
		t.Errorf("remux active = %d, want 0", sessions.RemuxActive())
	}
	if killed := sessions.KillAll(); killed != 0 {
		t.Errorf("KillAll killed %d of nothing, want 0", killed)
	}
}

// Attach exists so a slot refuses before a process exists; a slot that never
// gets one is invisible to the kill.
func TestAnUnattachedSlotIsSkippedByTheKill(t *testing.T) {
	sessions := NewSessions(0, nil)
	_ = sessions.Acquire(SessionRemux, "never started")
	if sessions.Active() != 1 {
		t.Fatalf("active = %d, want the reserved slot", sessions.Active())
	}
	if killed := sessions.KillAll(); killed != 0 {
		t.Errorf("KillAll killed %d processes from a slot that never had one", killed)
	}
}
