package main

import "time"

const (
	pluginID     = "com.scientia.resource-queue"
	botUsername  = "resource-queue"
	maxHistory   = 200
	maxQueueSize = 50
	maxResources = 100
	maxVarKeyLen = 64
	maxVarValLen = 256
	maxNameLen   = 100
	maxIPLen     = 45 // IPv6
	maxDescLen   = 500
	maxPurposeLen = 200
)

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

type Booking struct {
	ResourceID    string    `json:"resource_id"`
	UserID        string    `json:"user_id"`
	Purpose       string    `json:"purpose,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	NotifiedSoon  bool      `json:"notified_soon"`
	NotifiedQueue bool      `json:"notified_queue"`
}

func (b *Booking) IsExpired() bool {
	return time.Now().After(b.ExpiresAt)
}

type QueueEntry struct {
	UserID          string        `json:"user_id"`
	DesiredDuration time.Duration `json:"desired_duration"`
	Purpose         string        `json:"purpose,omitempty"`
	QueuedAt        time.Time     `json:"queued_at"`
}

type HistoryEntry struct {
	UserID     string    `json:"user_id"`
	ResourceID string    `json:"resource_id"`
	Purpose    string    `json:"purpose,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
}

// API response types

type BookingView struct {
	Booking
	Username string `json:"username"`
}

type QueueView struct {
	QueueEntry
	Username string `json:"username"`
}

type ResourceStatus struct {
	Resource     Resource     `json:"resource"`
	Booking      *BookingView `json:"booking,omitempty"`
	Queue        []QueueView  `json:"queue"`
	Subscribers  int          `json:"subscribers"`
	IsSubscribed bool         `json:"is_subscribed"`
	IsHolder     bool         `json:"is_holder"`
	InQueue      bool         `json:"in_queue"`
}

type StatusResponse struct {
	UserID   string           `json:"user_id"`
	IsAdmin  bool             `json:"is_admin"`
	Statuses []ResourceStatus `json:"statuses"`
}

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
}
