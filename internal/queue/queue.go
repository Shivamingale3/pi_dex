// Package queue persists notifications that could not be delivered and
// re-sends them, in order, once delivery works again.
package queue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

// DefaultMax is the maximum number of queued events. Older events are dropped
// (and counted) once the queue is full.
const DefaultMax = 500

// record is one line of the JSONL file: either a queued event or the count of
// events dropped because the queue was full.
type record struct {
	Event   *core.Event `json:"event,omitempty"`
	Dropped int         `json:"dropped,omitempty"`
}

// Queue is a FIFO of events backed by a JSONL file. Every mutation rewrites
// the file atomically (temp file + rename). With an empty path it only keeps
// events in memory.
type Queue struct {
	mu      sync.Mutex
	path    string
	max     int
	entries []entry
	nextSeq uint64
	dropped int
}

// entry pairs an event with an in-memory sequence number, so the flusher can
// remove exactly the event it sent even if the cap dropped others meanwhile.
type entry struct {
	seq   uint64
	event core.Event
}

// Open loads the queue stored at path, creating nothing until the first write.
// Unreadable lines are skipped. A missing file is an empty queue.
func Open(path string, max int) (*Queue, error) {
	if max <= 0 {
		max = DefaultMax
	}
	q := &Queue{path: path, max: max}
	if path == "" {
		return q, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return q, nil
	}
	if err != nil {
		return q, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("queue: skipping unreadable line in %s: %v", path, err)
			continue
		}
		q.dropped += r.Dropped
		if r.Event != nil {
			q.appendLocked(*r.Event)
		}
	}
	q.trimLocked()
	return q, scanner.Err()
}

// Len returns the number of queued events.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

// Dropped returns how many events were discarded because the queue was full.
func (q *Queue) Dropped() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}

// Push appends an event to the back of the queue. The event stays queued in
// memory even if persisting it fails; the error is returned for logging.
func (q *Queue) Push(e core.Event) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	e.Delayed = false
	q.appendLocked(e)
	q.trimLocked()
	return q.persistLocked()
}

// Front returns the oldest queued event and an id to pass to Remove.
func (q *Queue) Front() (core.Event, uint64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 {
		return core.Event{}, 0, false
	}
	return q.entries[0].event, q.entries[0].seq, true
}

// Remove deletes the event with the given id (from Front). It is a no-op if
// the event is already gone, e.g. dropped by the cap.
func (q *Queue) Remove(id uint64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, e := range q.entries {
		if e.seq == id {
			q.entries = append(q.entries[:i:i], q.entries[i+1:]...)
			return q.persistLocked()
		}
	}
	return nil
}

// ResetDropped clears the dropped counter after its summary was delivered.
func (q *Queue) ResetDropped() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dropped = 0
	return q.persistLocked()
}

func (q *Queue) appendLocked(e core.Event) {
	q.nextSeq++
	q.entries = append(q.entries, entry{seq: q.nextSeq, event: e})
}

func (q *Queue) trimLocked() {
	if over := len(q.entries) - q.max; over > 0 {
		q.entries = append([]entry(nil), q.entries[over:]...)
		q.dropped += over
	}
}

func (q *Queue) persistLocked() error {
	if q.path == "" {
		return nil
	}
	if len(q.entries) == 0 && q.dropped == 0 {
		if err := os.Remove(q.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if q.dropped > 0 {
		if err := enc.Encode(record{Dropped: q.dropped}); err != nil {
			return err
		}
	}
	for i := range q.entries {
		if err := enc.Encode(record{Event: &q.entries[i].event}); err != nil {
			return err
		}
	}
	return writeAtomic(q.path, buf.Bytes())
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s: %w", tmpName, err)
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
