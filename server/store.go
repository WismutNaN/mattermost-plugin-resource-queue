package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	keyResourceList = "res_list"
	prefixResource  = "res:"
	prefixBooking   = "bk:"
	prefixQueue     = "q:"
	prefixSubs      = "sub:"
	prefixHistory   = "hist:"
	keyBotUserID    = "bot_uid"
)

type Store struct {
	api plugin.API
}

func NewStore(api plugin.API) *Store {
	return &Store{api: api}
}

// --- helpers ---

func (s *Store) get(key string, v interface{}) error {
	data, appErr := s.api.KVGet(key)
	if appErr != nil {
		return fmt.Errorf("kvget %s: %v", key, appErr)
	}
	if data == nil {
		return nil // caller checks for zero value
	}
	return json.Unmarshal(data, v)
}

func (s *Store) set(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if appErr := s.api.KVSet(key, data); appErr != nil {
		return fmt.Errorf("kvset %s: %v", key, appErr)
	}
	return nil
}

func (s *Store) del(key string) {
	s.api.KVDelete(key)
}

// --- Resource list ---

func (s *Store) getResourceIDs() ([]string, error) {
	var ids []string
	if err := s.get(keyResourceList, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Store) setResourceIDs(ids []string) error {
	return s.set(keyResourceList, ids)
}

// --- Resources ---

func (s *Store) GetResource(id string) (*Resource, error) {
	var r Resource
	if err := s.get(prefixResource+id, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, nil
	}
	return &r, nil
}

func (s *Store) SaveResource(r *Resource) error {
	if err := s.set(prefixResource+r.ID, r); err != nil {
		return err
	}
	ids, _ := s.getResourceIDs()
	for _, id := range ids {
		if id == r.ID {
			return nil
		}
	}
	if len(ids) >= maxResources {
		return fmt.Errorf("max resources limit (%d) reached", maxResources)
	}
	ids = append(ids, r.ID)
	return s.setResourceIDs(ids)
}

func (s *Store) DeleteResource(id string) error {
	s.del(prefixResource + id)
	s.del(prefixBooking + id)
	s.del(prefixQueue + id)
	s.del(prefixSubs + id)
	s.del(prefixHistory + id)

	ids, _ := s.getResourceIDs()
	filtered := make([]string, 0, len(ids))
	for _, eid := range ids {
		if eid != id {
			filtered = append(filtered, eid)
		}
	}
	return s.setResourceIDs(filtered)
}

func (s *Store) GetAllResources() ([]*Resource, error) {
	ids, err := s.getResourceIDs()
	if err != nil {
		return nil, err
	}
	out := make([]*Resource, 0, len(ids))
	for _, id := range ids {
		r, err := s.GetResource(id)
		if err == nil && r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// --- Bookings ---

// GetBooking returns the active booking or nil. Does NOT auto-delete expired.
func (s *Store) GetBooking(resourceID string) (*Booking, error) {
	var b Booking
	if err := s.get(prefixBooking+resourceID, &b); err != nil {
		return nil, err
	}
	if b.ResourceID == "" {
		return nil, nil
	}
	if b.IsExpired() {
		return nil, nil
	}
	return &b, nil
}

// GetBookingRaw returns booking even if expired (for scheduler cleanup).
func (s *Store) GetBookingRaw(resourceID string) (*Booking, error) {
	var b Booking
	if err := s.get(prefixBooking+resourceID, &b); err != nil {
		return nil, err
	}
	if b.ResourceID == "" {
		return nil, nil
	}
	return &b, nil
}

func (s *Store) SaveBooking(b *Booking) error {
	return s.set(prefixBooking+b.ResourceID, b)
}

func (s *Store) DeleteBooking(resourceID string) {
	s.del(prefixBooking + resourceID)
}

// --- Queue ---

type queueData struct {
	Entries []QueueEntry `json:"entries"`
}

func (s *Store) getQueue(resourceID string) (*queueData, error) {
	var q queueData
	if err := s.get(prefixQueue+resourceID, &q); err != nil {
		return nil, err
	}
	if q.Entries == nil {
		q.Entries = []QueueEntry{}
	}
	return &q, nil
}

func (s *Store) GetQueueEntries(resourceID string) ([]QueueEntry, error) {
	q, err := s.getQueue(resourceID)
	if err != nil {
		return nil, err
	}
	return q.Entries, nil
}

func (s *Store) AddToQueue(resourceID string, entry QueueEntry) (int, error) {
	q, err := s.getQueue(resourceID)
	if err != nil {
		return 0, err
	}
	for _, e := range q.Entries {
		if e.UserID == entry.UserID {
			return -1, fmt.Errorf("already in queue")
		}
	}
	if len(q.Entries) >= maxQueueSize {
		return -1, fmt.Errorf("queue is full (max %d)", maxQueueSize)
	}
	q.Entries = append(q.Entries, entry)
	if err := s.set(prefixQueue+resourceID, q); err != nil {
		return 0, err
	}
	return len(q.Entries), nil
}

func (s *Store) RemoveFromQueue(resourceID, userID string) {
	q, err := s.getQueue(resourceID)
	if err != nil {
		return
	}
	filtered := make([]QueueEntry, 0, len(q.Entries))
	for _, e := range q.Entries {
		if e.UserID != userID {
			filtered = append(filtered, e)
		}
	}
	q.Entries = filtered
	s.set(prefixQueue+resourceID, q)
}

func (s *Store) PopQueue(resourceID string) (*QueueEntry, error) {
	q, err := s.getQueue(resourceID)
	if err != nil || len(q.Entries) == 0 {
		return nil, err
	}
	first := q.Entries[0]
	q.Entries = q.Entries[1:]
	if err := s.set(prefixQueue+resourceID, q); err != nil {
		return nil, err
	}
	return &first, nil
}

// --- Subscriptions ---

type subsData struct {
	UserIDs []string `json:"user_ids"`
}

func (s *Store) getSubs(resourceID string) (*subsData, error) {
	var sd subsData
	if err := s.get(prefixSubs+resourceID, &sd); err != nil {
		return nil, err
	}
	if sd.UserIDs == nil {
		sd.UserIDs = []string{}
	}
	return &sd, nil
}

func (s *Store) GetSubscribers(resourceID string) ([]string, error) {
	sd, err := s.getSubs(resourceID)
	if err != nil {
		return nil, err
	}
	return sd.UserIDs, nil
}

func (s *Store) IsSubscribed(resourceID, userID string) bool {
	subs, _ := s.GetSubscribers(resourceID)
	for _, uid := range subs {
		if uid == userID {
			return true
		}
	}
	return false
}

func (s *Store) Subscribe(resourceID, userID string) error {
	sd, _ := s.getSubs(resourceID)
	for _, uid := range sd.UserIDs {
		if uid == userID {
			return fmt.Errorf("already subscribed")
		}
	}
	sd.UserIDs = append(sd.UserIDs, userID)
	return s.set(prefixSubs+resourceID, sd)
}

func (s *Store) Unsubscribe(resourceID, userID string) {
	sd, _ := s.getSubs(resourceID)
	filtered := make([]string, 0, len(sd.UserIDs))
	for _, uid := range sd.UserIDs {
		if uid != userID {
			filtered = append(filtered, uid)
		}
	}
	sd.UserIDs = filtered
	s.set(prefixSubs+resourceID, sd)
}

// --- History ---

type historyData struct {
	Entries []HistoryEntry `json:"entries"`
}

func (s *Store) AddHistory(entry HistoryEntry) error {
	var h historyData
	s.get(prefixHistory+entry.ResourceID, &h)
	if h.Entries == nil {
		h.Entries = []HistoryEntry{}
	}
	h.Entries = append(h.Entries, entry)
	if len(h.Entries) > maxHistory {
		h.Entries = h.Entries[len(h.Entries)-maxHistory:]
	}
	return s.set(prefixHistory+entry.ResourceID, &h)
}

func (s *Store) GetHistory(resourceID string, limit int) ([]HistoryEntry, error) {
	var h historyData
	if err := s.get(prefixHistory+resourceID, &h); err != nil {
		return nil, err
	}
	if h.Entries == nil {
		return []HistoryEntry{}, nil
	}
	sort.Slice(h.Entries, func(i, j int) bool {
		return h.Entries[i].StartedAt.After(h.Entries[j].StartedAt)
	})
	if limit > 0 && len(h.Entries) > limit {
		h.Entries = h.Entries[:limit]
	}
	return h.Entries, nil
}
