// Package workload coordinates optional background CPU work with interactive
// media delivery. Playback always wins; a preview can be built again later.
package workload

import (
	"context"
	"errors"
	"sync"
)

// ErrPreempted is the cancellation cause delivered to background work when an
// interactive job starts.
var ErrPreempted = errors.New("background work preempted by playback")

type Coordinator struct {
	mu          sync.Mutex
	interactive int
	nextID      uint64
	background  map[uint64]context.CancelCauseFunc
	changed     chan struct{}
}

func New() *Coordinator {
	return &Coordinator{background: map[uint64]context.CancelCauseFunc{}, changed: make(chan struct{})}
}

// BeginInteractive cancels every optional job and prevents another from
// starting until the returned release function is called.
func (c *Coordinator) BeginInteractive() func() {
	c.mu.Lock()
	c.interactive++
	for _, cancel := range c.background {
		cancel(ErrPreempted)
	}
	c.signalLocked()
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.interactive--
			c.signalLocked()
			c.mu.Unlock()
		})
	}
}

// BeginBackground waits for an idle playback window and returns a context that
// is cancelled as soon as playback needs the machine.
func (c *Coordinator) BeginBackground(ctx context.Context) (context.Context, func(), error) {
	for {
		c.mu.Lock()
		if c.interactive == 0 {
			c.nextID++
			id := c.nextID
			jobCtx, cancel := context.WithCancelCause(ctx)
			c.background[id] = cancel
			c.mu.Unlock()
			var once sync.Once
			return jobCtx, func() {
				once.Do(func() {
					c.mu.Lock()
					delete(c.background, id)
					c.signalLocked()
					c.mu.Unlock()
					cancel(nil)
				})
			}, nil
		}
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-changed:
		}
	}
}

func (c *Coordinator) signalLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}
