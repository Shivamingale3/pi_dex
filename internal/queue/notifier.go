package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

const (
	DefaultFlushInterval = 30 * time.Second
	// Pause between queued messages so a large backlog stays under
	// Telegram's per-chat rate limit.
	DefaultFlushGap = time.Second
)

// Notifier wraps another notifier with the offline queue:
//   - while the queue is non-empty, new events go to its back (order is kept);
//   - when a direct send fails, the event is queued instead of lost;
//   - Run flushes the queue in FIFO order every FlushInterval.
type Notifier struct {
	inner       core.Notifier
	queue       *Queue
	isPermanent func(error) bool

	FlushInterval time.Duration
	FlushGap      time.Duration

	// sendMu serialises delivery so queued and live events cannot interleave.
	sendMu sync.Mutex
	sleep  func(time.Duration)
}

// NewNotifier returns a queueing notifier. isPermanent reports errors for which
// a message can never be delivered; such events are logged and discarded
// instead of blocking the queue. It may be nil.
func NewNotifier(inner core.Notifier, q *Queue, isPermanent func(error) bool) *Notifier {
	if isPermanent == nil {
		isPermanent = func(error) bool { return false }
	}
	return &Notifier{
		inner:         inner,
		queue:         q,
		isPermanent:   isPermanent,
		FlushInterval: DefaultFlushInterval,
		FlushGap:      DefaultFlushGap,
		sleep:         time.Sleep,
	}
}

// Send delivers the event now, or queues it when the queue already holds
// older events or delivery fails. A queued event is not an error.
func (n *Notifier) Send(event core.Event) error {
	if n.queue.Len() > 0 {
		return n.enqueue(event, "queue not empty")
	}

	n.sendMu.Lock()
	defer n.sendMu.Unlock()

	if n.queue.Len() > 0 {
		return n.enqueue(event, "queue not empty")
	}
	err := n.inner.Send(event)
	if err == nil {
		return nil
	}
	if n.isPermanent(err) {
		return err
	}
	return n.enqueue(event, err.Error())
}

func (n *Notifier) enqueue(event core.Event, reason string) error {
	if err := n.queue.Push(event); err != nil {
		log.Printf("queue: %s queued in memory only (persist failed: %v)", event.EventType, err)
	} else {
		log.Printf("queue: %s queued (%s), %d pending", event.EventType, reason, n.queue.Len())
	}
	return nil
}

// Run flushes the queue every FlushInterval until ctx is cancelled. The first
// flush happens immediately so a queue loaded from disk drains quickly.
func (n *Notifier) Run(ctx context.Context) {
	for {
		if sent, err := n.Flush(); err != nil {
			log.Printf("queue: flush stopped after %d sent, %d pending: %v", sent, n.queue.Len(), err)
		} else if sent > 0 {
			log.Printf("queue: flushed %d queued notifications", sent)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.FlushInterval):
		}
	}
}

// Flush sends queued events oldest first and stops at the first failure.
// A dropped-events summary, if any, is sent first.
func (n *Notifier) Flush() (sent int, err error) {
	if n.queue.Len() == 0 && n.queue.Dropped() == 0 {
		return 0, nil
	}

	n.sendMu.Lock()
	defer n.sendMu.Unlock()

	if dropped := n.queue.Dropped(); dropped > 0 {
		summary := core.Event{
			Source:    core.SourceQueue,
			EventType: core.EventAlertsDropped,
			Severity:  core.SeverityWarn,
			Title:     "Alerts Dropped",
			Message:   fmt.Sprintf("%d alerts dropped while offline", dropped),
			Timestamp: time.Now(),
		}
		if err := n.inner.Send(summary); err != nil && !n.isPermanent(err) {
			return sent, err
		}
		if err := n.queue.ResetDropped(); err != nil {
			log.Printf("queue: persist: %v", err)
		}
		sent++
	}

	for {
		event, id, ok := n.queue.Front()
		if !ok {
			return sent, nil
		}
		if sent > 0 {
			n.sleep(n.FlushGap)
		}
		event.Delayed = true
		if err := n.inner.Send(event); err != nil {
			if !n.isPermanent(err) {
				return sent, err
			}
			log.Printf("queue: discarding undeliverable %s: %v", event.EventType, err)
		} else {
			sent++
		}
		if err := n.queue.Remove(id); err != nil {
			log.Printf("queue: persist: %v", err)
		}
	}
}
