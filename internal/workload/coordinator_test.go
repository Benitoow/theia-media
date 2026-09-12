package workload

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInteractivePreemptsAndBlocksBackground(t *testing.T) {
	c := New()
	background, done, err := c.BeginBackground(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	releasePlayback := c.BeginInteractive()
	if !errors.Is(context.Cause(background), ErrPreempted) {
		t.Fatalf("cause = %v, want ErrPreempted", context.Cause(background))
	}
	done()

	started := make(chan struct{})
	go func() {
		_, release, err := c.BeginBackground(context.Background())
		if err == nil {
			close(started)
			release()
		}
	}()
	select {
	case <-started:
		t.Fatal("background started during playback")
	case <-time.After(20 * time.Millisecond):
	}
	releasePlayback()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background did not resume after playback")
	}
}
