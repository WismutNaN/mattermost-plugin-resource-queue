package main

import "time"

// Resource represents a managed resource (PC, server, service)
type Resource struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	IP          string            `json:"ip,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Description string            `json:"description,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CreatedBy   string            `json:"created_by"`
}

// Booking represents an active resource booking
type Booking struct {
	ResourceID    string    `json:"resource_id"`
	UserID        string    `json:"user_id"`
	Purpose       string    `json:"purpose,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	NotifiedSoon  bool      `json:"notified_soon"`
	NotifiedQueue bool      `json:"notified_queue"`
}

// QueueEntry represents a user waiting in queue for a resource
type QueueEntry struct {
	UserID          string        `json:"user_id"`
	DesiredDuration time.Duration `json:"desired_duration"`
	Purpose         string        `json:"purpose,omitempty"`
	QueuedAt        time.Time     `json:"queued_at"`
}

// Queue is a list of queue entries
type Queue struct {
	ResourceID string       `json:"resource_id"`
	Entries    []QueueEntry `json:"entries"`
}

// Subscription tracks users subscribed to resource status changes
type Subscription struct {
	ResourceID string   `json:"resource_id"`
	UserIDs    []string `json:"user_ids"`
}

// HistoryEntry records past bookings
type HistoryEntry struct {
	UserID     string    `json:"user_id"`
	ResourceID string    `json:"resource_id"`
	Purpose    string    `json:"purpose,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
}

// History stores history entries (chunked by resource)
type History struct {
	ResourceID string         `json:"resource_id"`
	Entries    []HistoryEntry `json:"entries"`
}

// ResourceStatus is a combined view for API responses
type ResourceStatus struct {
	Resource     Resource     `json:"resource"`
	Booking      *BookingView `json:"booking,omitempty"`
	Queue        []QueueView  `json:"queue"`
	Subscribers  int          `json:"subscribers"`
	IsSubscribed bool         `json:"is_subscribed"`
	IsHolder     bool         `json:"is_holder"`
	InQueue      bool         `json:"in_queue"`
}

// BookingView is Booking with resolved username
type BookingView struct {
	Booking
	Username string `json:"username"`
}

// QueueView is QueueEntry with resolved username
type QueueView struct {
	QueueEntry
	Username string `json:"username"`
}

// Duration presets for the UI
type DurationPreset struct {
	Label   string `json:"label"`
	Minutes int    `json:"minutes"`
}

var DefaultPresets = []DurationPreset{
	{Label: "30 мин", Minutes: 30},
	{Label: "1 час", Minutes: 60},
	{Label: "2 часа", Minutes: 120},
	{Label: "4 часа", Minutes: 240},
	{Label: "8 часов", Minutes: 480},
	{Label: "До конца дня", Minutes: 600},
}
