package db

import (
	"bytes"
	"encoding/gob"
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
)

// ModifiedTasks allows subscribers to keep track of Tasks that have been
// modified. It implements StartTrackingModifiedTasks and GetModifiedTasks from
// the DB interface.
type ModifiedTasks struct {
	// map[subscriber_id][]task_gob
	tasks map[string][][]byte
	// After the expiration time, subscribers are automatically removed.
	expiration map[string]time.Time
	// Protects tasks and expiration.
	mtx sync.RWMutex
}

// See docs for DB interface.
func (m *ModifiedTasks) GetModifiedTasks(id string) ([]*Task, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.expiration[id]; !ok {
		return nil, ErrUnknownId
	}
	d := TaskDecoder{}
	for _, g := range m.tasks[id] {
		if !d.Process(g) {
			break
		}
	}
	rv, err := d.Result()
	if err != nil {
		return nil, err
	}
	m.expiration[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	delete(m.tasks, id)
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// clearExpiredSubscribers periodically deletes data about any subscribers that
// haven't been seen within MODIFIED_BUILDS_TIMEOUT. Must be called as a
// goroutine. Returns when there are no remaining subscribers.
func (m *ModifiedTasks) clearExpiredSubscribers() {
	for _ = range time.Tick(time.Minute) {
		m.mtx.Lock()
		for id, t := range m.expiration {
			if time.Now().After(t) {
				delete(m.tasks, id)
				delete(m.expiration, id)
			}
		}
		anyLeft := len(m.expiration) > 0
		m.mtx.Unlock()
		if !anyLeft {
			break
		}
	}
}

// TrackModifiedTask indicates the given Task should be returned from the next
// call to GetModifiedTasks from each subscriber.
func (m *ModifiedTasks) TrackModifiedTask(t *Task) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		glog.Fatal(err)
	}
	m.TrackModifiedTasksGOB([][]byte{buf.Bytes()})
}

// TrackModifiedTasksGOB is a batch, GOB version of TrackModifiedTask. It is
// equivalent to GOB-decoding each element of gobs as a Task and calling
// TrackModifiedTask on each one. Contents of gobs must not be modified after
// this call.
func (m *ModifiedTasks) TrackModifiedTasksGOB(gobs [][]byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for id, _ := range m.expiration {
		m.tasks[id] = append(m.tasks[id], gobs...)
	}
}

// See docs for DB interface.
func (m *ModifiedTasks) StartTrackingModifiedTasks() (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if len(m.expiration) == 0 {
		// Initialize the data structure and start expiration goroutine.
		m.tasks = map[string][][]byte{}
		m.expiration = map[string]time.Time{}
		go m.clearExpiredSubscribers()
	} else if len(m.expiration) >= MAX_MODIFIED_BUILDS_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	m.expiration[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return id, nil
}
