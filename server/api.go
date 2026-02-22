package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) setupAPI() {
	// Authenticated API routes
	s := p.router.PathPrefix("/api/v1").Subrouter()
	s.Use(p.authMiddleware)

	// Resources CRUD
	s.HandleFunc("/resources", p.handleGetResources).Methods("GET")
	s.HandleFunc("/resources", p.handleCreateResource).Methods("POST")
	s.HandleFunc("/resources/{id}", p.handleGetResource).Methods("GET")
	s.HandleFunc("/resources/{id}", p.handleUpdateResource).Methods("PUT")
	s.HandleFunc("/resources/{id}", p.handleDeleteResource).Methods("DELETE")

	// Status (combined view)
	s.HandleFunc("/status", p.handleGetAllStatus).Methods("GET")
	s.HandleFunc("/status/{id}", p.handleGetResourceStatus).Methods("GET")

	// Bookings
	s.HandleFunc("/resources/{id}/book", p.handleBookResource).Methods("POST")
	s.HandleFunc("/resources/{id}/release", p.handleReleaseResource).Methods("POST")
	s.HandleFunc("/resources/{id}/extend", p.handleExtendResource).Methods("POST")

	// Queue
	s.HandleFunc("/resources/{id}/queue", p.handleJoinQueue).Methods("POST")
	s.HandleFunc("/resources/{id}/queue", p.handleLeaveQueue).Methods("DELETE")

	// Subscriptions
	s.HandleFunc("/resources/{id}/subscribe", p.handleSubscribe).Methods("POST")
	s.HandleFunc("/resources/{id}/unsubscribe", p.handleUnsubscribe).Methods("POST")

	// History
	s.HandleFunc("/resources/{id}/history", p.handleGetHistory).Methods("GET")

	// Presets
	s.HandleFunc("/presets", p.handleGetPresets).Methods("GET")

	// Interactive button actions (NO auth middleware — Mattermost server calls these directly)
	p.router.HandleFunc("/action/book", p.handleActionBook).Methods("POST")
	p.router.HandleFunc("/action/queue", p.handleActionQueue).Methods("POST")
}

func (p *Plugin) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (p *Plugin) isAdmin(userID string) bool {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false
	}
	return strings.Contains(user.Roles, "system_admin")
}

// --- Resources CRUD ---

func (p *Plugin) handleGetResources(w http.ResponseWriter, r *http.Request) {
	resources, err := p.store.GetAllResources()
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, resources)
}

func (p *Plugin) handleCreateResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if !p.isAdmin(userID) {
		respondError(w, 403, "Admin only")
		return
	}

	var res Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		respondError(w, 400, "Invalid JSON")
		return
	}

	if res.Name == "" {
		respondError(w, 400, "Name is required")
		return
	}

	res.ID = model.NewId()
	res.CreatedAt = time.Now()
	res.CreatedBy = userID
	if res.Variables == nil {
		res.Variables = map[string]string{}
	}

	if err := p.store.SaveResource(&res); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, res)
}

func (p *Plugin) handleGetResource(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := p.store.GetResource(id)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	if res == nil {
		respondError(w, 404, "Resource not found")
		return
	}
	respondJSON(w, res)
}

func (p *Plugin) handleUpdateResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if !p.isAdmin(userID) {
		respondError(w, 403, "Admin only")
		return
	}

	id := mux.Vars(r)["id"]
	existing, err := p.store.GetResource(id)
	if err != nil || existing == nil {
		respondError(w, 404, "Resource not found")
		return
	}

	var res Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		respondError(w, 400, "Invalid JSON")
		return
	}

	res.ID = id
	res.CreatedAt = existing.CreatedAt
	res.CreatedBy = existing.CreatedBy
	if res.Variables == nil {
		res.Variables = existing.Variables
	}

	if err := p.store.SaveResource(&res); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, res)
}

func (p *Plugin) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if !p.isAdmin(userID) {
		respondError(w, 403, "Admin only")
		return
	}

	id := mux.Vars(r)["id"]
	if err := p.store.DeleteResource(id); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, map[string]string{"status": "deleted"})
}

// --- Status ---

func (p *Plugin) handleGetAllStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	resources, err := p.store.GetAllResources()
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	statuses := make([]ResourceStatus, 0, len(resources))
	for _, res := range resources {
		statuses = append(statuses, p.buildResourceStatus(res, userID))
	}
	respondJSON(w, map[string]interface{}{
		"user_id":  userID,
		"is_admin": p.isAdmin(userID),
		"statuses": statuses,
	})
}

func (p *Plugin) handleGetResourceStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]
	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		respondError(w, 404, "Resource not found")
		return
	}
	respondJSON(w, p.buildResourceStatus(res, userID))
}

func (p *Plugin) buildResourceStatus(res *Resource, currentUserID string) ResourceStatus {
	booking, _ := p.store.GetBooking(res.ID)
	queue, _ := p.store.GetQueue(res.ID)
	subs, _ := p.store.GetSubscribers(res.ID)

	var bv *BookingView
	if booking != nil {
		bv = &BookingView{
			Booking:  *booking,
			Username: p.getUsername(booking.UserID),
		}
	}

	queueViews := make([]QueueView, 0, len(queue.Entries))
	for _, e := range queue.Entries {
		queueViews = append(queueViews, QueueView{
			QueueEntry: e,
			Username:   p.getUsername(e.UserID),
		})
	}

	// Determine if current user is subscribed
	isSubscribed := false
	for _, uid := range subs {
		if uid == currentUserID {
			isSubscribed = true
			break
		}
	}

	// Determine if current user is in queue
	inQueue := false
	for _, e := range queue.Entries {
		if e.UserID == currentUserID {
			inQueue = true
			break
		}
	}

	return ResourceStatus{
		Resource:     *res,
		Booking:      bv,
		Queue:        queueViews,
		Subscribers:  len(subs),
		IsSubscribed: isSubscribed,
		IsHolder:     booking != nil && booking.UserID == currentUserID,
		InQueue:      inQueue,
	}
}

// --- Bookings ---

type BookRequest struct {
	Minutes int    `json:"minutes"`
	Purpose string `json:"purpose,omitempty"`
}

func (p *Plugin) handleBookResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		respondError(w, 404, "Resource not found")
		return
	}

	var req BookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "Invalid JSON")
		return
	}
	if req.Minutes <= 0 {
		respondError(w, 400, "Invalid duration")
		return
	}
	maxMinutes := p.getMaxBookingHours() * 60
	if req.Minutes > maxMinutes {
		respondError(w, 400, fmt.Sprintf("Max booking duration is %d hours", p.getMaxBookingHours()))
		return
	}

	// Check if already booked
	existing, _ := p.store.GetBooking(id)
	if existing != nil {
		respondError(w, 409, "Resource is already booked")
		return
	}

	booking := &Booking{
		ResourceID: id,
		UserID:     userID,
		Purpose:    req.Purpose,
		StartedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Duration(req.Minutes) * time.Minute),
	}

	if err := p.store.SaveBooking(booking); err != nil {
		respondError(w, 500, err.Error())
		return
	}

	// Remove from queue if in queue
	p.store.RemoveFromQueue(id, userID)

	// Notify subscribers
	p.notifySubscribers(id, fmt.Sprintf("🔒 **%s** занят пользователем @%s на %s",
		res.Name, p.getUsername(userID), formatDuration(time.Duration(req.Minutes)*time.Minute)), userID)

	respondJSON(w, booking)
}

func (p *Plugin) handleReleaseResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		respondError(w, 404, "Resource not found")
		return
	}

	booking, _ := p.store.GetBooking(id)
	if booking == nil {
		respondError(w, 400, "Resource is not booked")
		return
	}

	// Only the booker or admin can release
	if booking.UserID != userID && !p.isAdmin(userID) {
		respondError(w, 403, "Only the current holder or admin can release")
		return
	}

	// Add to history
	p.store.AddHistory(HistoryEntry{
		UserID:     booking.UserID,
		ResourceID: id,
		Purpose:    booking.Purpose,
		StartedAt:  booking.StartedAt,
		EndedAt:    time.Now(),
	})

	p.store.DeleteBooking(id)

	// Notify subscribers
	p.notifySubscribers(id, fmt.Sprintf("🔓 **%s** освобождён", res.Name), "")

	// Process queue - notify next in line
	p.processQueue(id, res.Name)

	respondJSON(w, map[string]string{"status": "released"})
}

// --- Extend ---

type ExtendRequest struct {
	Minutes int `json:"minutes"`
}

func (p *Plugin) handleExtendResource(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		respondError(w, 404, "Resource not found")
		return
	}

	booking, _ := p.store.GetBooking(id)
	if booking == nil {
		respondError(w, 400, "Resource is not booked")
		return
	}
	if booking.UserID != userID {
		respondError(w, 403, "Only the current holder can extend")
		return
	}

	var req ExtendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Minutes <= 0 {
		respondError(w, 400, "Invalid duration")
		return
	}

	booking.ExpiresAt = booking.ExpiresAt.Add(time.Duration(req.Minutes) * time.Minute)
	booking.NotifiedSoon = false

	maxMinutes := p.getMaxBookingHours() * 60
	totalMinutes := int(booking.ExpiresAt.Sub(booking.StartedAt).Minutes())
	if totalMinutes > maxMinutes {
		respondError(w, 400, fmt.Sprintf("Total duration exceeds max (%d hours)", p.getMaxBookingHours()))
		return
	}

	if err := p.store.SaveBooking(booking); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, booking)
}

// --- Queue ---

type QueueRequest struct {
	Minutes int    `json:"minutes"`
	Purpose string `json:"purpose,omitempty"`
}

func (p *Plugin) handleJoinQueue(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		respondError(w, 404, "Resource not found")
		return
	}

	// Check if already holds the booking
	booking, _ := p.store.GetBooking(id)
	if booking != nil && booking.UserID == userID {
		respondError(w, 400, "You already hold this resource")
		return
	}

	var req QueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "Invalid JSON")
		return
	}

	entry := QueueEntry{
		UserID:          userID,
		DesiredDuration: time.Duration(req.Minutes) * time.Minute,
		Purpose:         req.Purpose,
		QueuedAt:        time.Now(),
	}

	pos, err := p.store.AddToQueue(id, entry)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}

	// Notify current holder that someone queued
	if booking != nil && !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**",
			p.getUsername(userID), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}

	respondJSON(w, map[string]interface{}{"position": pos})
}

func (p *Plugin) handleLeaveQueue(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	if err := p.store.RemoveFromQueue(id, userID); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, map[string]string{"status": "left queue"})
}

// --- Subscriptions ---

func (p *Plugin) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	if err := p.store.Subscribe(id, userID); err != nil {
		respondError(w, 400, err.Error())
		return
	}
	respondJSON(w, map[string]string{"status": "subscribed"})
}

func (p *Plugin) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	if err := p.store.Unsubscribe(id, userID); err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, map[string]string{"status": "unsubscribed"})
}

// --- History ---

func (p *Plugin) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	history, err := p.store.GetHistory(id)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	type HistoryView struct {
		HistoryEntry
		Username string `json:"username"`
	}
	views := make([]HistoryView, 0, len(history.Entries))
	for _, e := range history.Entries {
		views = append(views, HistoryView{
			HistoryEntry: e,
			Username:     p.getUsername(e.UserID),
		})
	}
	respondJSON(w, views)
}

// --- Presets ---

func (p *Plugin) handleGetPresets(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, DefaultPresets)
}

// --- Interactive Button Actions ---

func (p *Plugin) handleActionBook(w http.ResponseWriter, r *http.Request) {
	var req model.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.API.LogError("Action book: bad request", "error", err.Error())
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: неверный запрос"})
		return
	}

	userID := req.UserId
	resourceID, _ := req.Context["resource_id"].(string)
	minutesFloat, _ := req.Context["minutes"].(float64)
	minutes := int(minutesFloat)

	if resourceID == "" || minutes <= 0 || userID == "" {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: неверные параметры"})
		return
	}

	res, err := p.store.GetResource(resourceID)
	if err != nil || res == nil {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ресурс не найден"})
		return
	}

	existing, _ := p.store.GetBooking(resourceID)
	if existing != nil {
		respondJSON(w, &model.PostActionIntegrationResponse{
			EphemeralText: fmt.Sprintf("🔴 **%s** уже занят @%s. Используйте `/rq queue %s 1h`",
				res.Name, p.getUsername(existing.UserID), res.Name),
		})
		return
	}

	booking := &Booking{
		ResourceID: resourceID,
		UserID:     userID,
		StartedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Duration(minutes) * time.Minute),
	}
	if err := p.store.SaveBooking(booking); err != nil {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: " + err.Error()})
		return
	}

	p.store.RemoveFromQueue(resourceID, userID)
	p.notifySubscribers(resourceID, fmt.Sprintf("🔒 **%s** занят @%s на %s",
		res.Name, p.getUsername(userID), formatDuration(time.Duration(minutes)*time.Minute)), userID)

	respondJSON(w, &model.PostActionIntegrationResponse{
		EphemeralText: fmt.Sprintf("✅ Вы забронировали **%s** на %dм (до %s)",
			res.Name, minutes, booking.ExpiresAt.Format("15:04")),
	})
}

func (p *Plugin) handleActionQueue(w http.ResponseWriter, r *http.Request) {
	var req model.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: неверный запрос"})
		return
	}

	userID := req.UserId
	resourceID, _ := req.Context["resource_id"].(string)
	minutesFloat, _ := req.Context["minutes"].(float64)
	minutes := int(minutesFloat)
	if minutes <= 0 {
		minutes = 60
	}

	if resourceID == "" || userID == "" {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: неверные параметры"})
		return
	}

	res, err := p.store.GetResource(resourceID)
	if err != nil || res == nil {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ресурс не найден"})
		return
	}

	// If free, suggest booking
	booking, _ := p.store.GetBooking(resourceID)
	if booking == nil {
		respondJSON(w, &model.PostActionIntegrationResponse{
			EphemeralText: fmt.Sprintf("**%s** свободен! Используйте `/rq book %s 1h`", res.Name, res.Name),
		})
		return
	}

	if booking.UserID == userID {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Вы уже занимаете этот ресурс."})
		return
	}

	entry := QueueEntry{
		UserID:          userID,
		DesiredDuration: time.Duration(minutes) * time.Minute,
		QueuedAt:        time.Now(),
	}

	pos, err := p.store.AddToQueue(resourceID, entry)
	if err != nil {
		respondJSON(w, &model.PostActionIntegrationResponse{EphemeralText: "Ошибка: " + err.Error()})
		return
	}

	if !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**", p.getUsername(userID), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}

	respondJSON(w, &model.PostActionIntegrationResponse{
		EphemeralText: fmt.Sprintf("✅ Вы в очереди на **%s** (позиция: %d)", res.Name, pos),
	})
}
