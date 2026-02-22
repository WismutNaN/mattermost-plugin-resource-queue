package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	keyResource     = "resource:%s"
	keyResourceList = "resource_list"
	keyBooking      = "booking:%s"
	keyQueue        = "queue:%s"
	keySubs         = "subs:%s"
	keyHistory      = "history:%s"
)

type Store struct {
	api plugin.API
}

func NewStore(api plugin.API) *Store {
	return &Store{api: api}
}

// --- Resources ---

func (s *Store) GetResourceList() ([]string, error) {
	data, appErr := s.api.KVGet(keyResourceList)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet resource_list: %v", appErr)
	}
	if data == nil {
		return []string{}, nil
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Store) setResourceList(ids []string) error {
	data, _ := json.Marshal(ids)
	if appErr := s.api.KVSet(keyResourceList, data); appErr != nil {
		return fmt.Errorf("KVSet resource_list: %v", appErr)
	}
	return nil
}

func (s *Store) SaveResource(r *Resource) error {
	data, _ := json.Marshal(r)
	key := fmt.Sprintf(keyResource, r.ID)
	if appErr := s.api.KVSet(key, data); appErr != nil {
		return fmt.Errorf("KVSet resource: %v", appErr)
	}
	ids, _ := s.GetResourceList()
	found := false
	for _, id := range ids {
		if id == r.ID {
			found = true
			break
		}
	}
	if !found {
		ids = append(ids, r.ID)
		return s.setResourceList(ids)
	}
	return nil
}

func (s *Store) GetResource(id string) (*Resource, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keyResource, id))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet resource %s: %v", id, appErr)
	}
	if data == nil {
		return nil, nil
	}
	var r Resource
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) DeleteResource(id string) error {
	s.api.KVDelete(fmt.Sprintf(keyResource, id))
	s.api.KVDelete(fmt.Sprintf(keyBooking, id))
	s.api.KVDelete(fmt.Sprintf(keyQueue, id))
	s.api.KVDelete(fmt.Sprintf(keySubs, id))

	ids, _ := s.GetResourceList()
	newIDs := make([]string, 0, len(ids))
	for _, eid := range ids {
		if eid != id {
			newIDs = append(newIDs, eid)
		}
	}
	return s.setResourceList(newIDs)
}

func (s *Store) GetAllResources() ([]*Resource, error) {
	ids, err := s.GetResourceList()
	if err != nil {
		return nil, err
	}
	resources := make([]*Resource, 0, len(ids))
	for _, id := range ids {
		r, err := s.GetResource(id)
		if err != nil {
			continue
		}
		if r != nil {
			resources = append(resources, r)
		}
	}
	return resources, nil
}

// --- Bookings ---

func (s *Store) GetBooking(resourceID string) (*Booking, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keyBooking, resourceID))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet booking: %v", appErr)
	}
	if data == nil {
		return nil, nil
	}
	var b Booking
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	// Return nil for expired bookings but DON'T delete — scheduler handles cleanup
	if time.Now().After(b.ExpiresAt) {
		return nil, nil
	}
	return &b, nil
}

// GetBookingRaw returns booking even if expired (for scheduler to process expiry properly)
func (s *Store) GetBookingRaw(resourceID string) (*Booking, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keyBooking, resourceID))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet booking: %v", appErr)
	}
	if data == nil {
		return nil, nil
	}
	var b Booking
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) SaveBooking(b *Booking) error {
	data, _ := json.Marshal(b)
	if appErr := s.api.KVSet(fmt.Sprintf(keyBooking, b.ResourceID), data); appErr != nil {
		return fmt.Errorf("KVSet booking: %v", appErr)
	}
	return nil
}

func (s *Store) DeleteBooking(resourceID string) error {
	if appErr := s.api.KVDelete(fmt.Sprintf(keyBooking, resourceID)); appErr != nil {
		return fmt.Errorf("KVDelete booking: %v", appErr)
	}
	return nil
}

// --- Queue ---

func (s *Store) GetQueue(resourceID string) (*Queue, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keyQueue, resourceID))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet queue: %v", appErr)
	}
	if data == nil {
		return &Queue{ResourceID: resourceID, Entries: []QueueEntry{}}, nil
	}
	var q Queue
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) SaveQueue(q *Queue) error {
	data, _ := json.Marshal(q)
	if appErr := s.api.KVSet(fmt.Sprintf(keyQueue, q.ResourceID), data); appErr != nil {
		return fmt.Errorf("KVSet queue: %v", appErr)
	}
	return nil
}

func (s *Store) AddToQueue(resourceID string, entry QueueEntry) (int, error) {
	q, err := s.GetQueue(resourceID)
	if err != nil {
		return 0, err
	}
	// Check if already in queue
	for _, e := range q.Entries {
		if e.UserID == entry.UserID {
			return -1, fmt.Errorf("already in queue")
		}
	}
	q.Entries = append(q.Entries, entry)
	if err := s.SaveQueue(q); err != nil {
		return 0, err
	}
	return len(q.Entries), nil
}

func (s *Store) RemoveFromQueue(resourceID, userID string) error {
	q, err := s.GetQueue(resourceID)
	if err != nil {
		return err
	}
	newEntries := make([]QueueEntry, 0)
	for _, e := range q.Entries {
		if e.UserID != userID {
			newEntries = append(newEntries, e)
		}
	}
	q.Entries = newEntries
	return s.SaveQueue(q)
}

func (s *Store) PopQueue(resourceID string) (*QueueEntry, error) {
	q, err := s.GetQueue(resourceID)
	if err != nil {
		return nil, err
	}
	if len(q.Entries) == 0 {
		return nil, nil
	}
	first := q.Entries[0]
	q.Entries = q.Entries[1:]
	if err := s.SaveQueue(q); err != nil {
		return nil, err
	}
	return &first, nil
}

// --- Subscriptions ---

func (s *Store) GetSubscribers(resourceID string) ([]string, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keySubs, resourceID))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet subs: %v", appErr)
	}
	if data == nil {
		return []string{}, nil
	}
	var sub Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, err
	}
	return sub.UserIDs, nil
}

func (s *Store) Subscribe(resourceID, userID string) error {
	subs, _ := s.GetSubscribers(resourceID)
	for _, uid := range subs {
		if uid == userID {
			return fmt.Errorf("already subscribed")
		}
	}
	subs = append(subs, userID)
	sub := Subscription{ResourceID: resourceID, UserIDs: subs}
	data, _ := json.Marshal(sub)
	if appErr := s.api.KVSet(fmt.Sprintf(keySubs, resourceID), data); appErr != nil {
		return fmt.Errorf("KVSet subs: %v", appErr)
	}
	return nil
}

func (s *Store) Unsubscribe(resourceID, userID string) error {
	subs, _ := s.GetSubscribers(resourceID)
	newSubs := make([]string, 0)
	for _, uid := range subs {
		if uid != userID {
			newSubs = append(newSubs, uid)
		}
	}
	sub := Subscription{ResourceID: resourceID, UserIDs: newSubs}
	data, _ := json.Marshal(sub)
	if appErr := s.api.KVSet(fmt.Sprintf(keySubs, resourceID), data); appErr != nil {
		return fmt.Errorf("KVSet subs: %v", appErr)
	}
	return nil
}

// --- History ---

func (s *Store) AddHistory(entry HistoryEntry) error {
	h, err := s.GetHistory(entry.ResourceID)
	if err != nil {
		h = &History{ResourceID: entry.ResourceID, Entries: []HistoryEntry{}}
	}
	h.Entries = append(h.Entries, entry)
	// Keep last 500 entries
	if len(h.Entries) > 500 {
		h.Entries = h.Entries[len(h.Entries)-500:]
	}
	data, _ := json.Marshal(h)
	if appErr := s.api.KVSet(fmt.Sprintf(keyHistory, entry.ResourceID), data); appErr != nil {
		return fmt.Errorf("KVSet history: %v", appErr)
	}
	return nil
}

func (s *Store) GetHistory(resourceID string) (*History, error) {
	data, appErr := s.api.KVGet(fmt.Sprintf(keyHistory, resourceID))
	if appErr != nil {
		return nil, fmt.Errorf("KVGet history: %v", appErr)
	}
	if data == nil {
		return &History{ResourceID: resourceID, Entries: []HistoryEntry{}}, nil
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	// Sort by started_at desc
	sort.Slice(h.Entries, func(i, j int) bool {
		return h.Entries[i].StartedAt.After(h.Entries[j].StartedAt)
	})
	return &h, nil
}
