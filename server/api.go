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

// initRoutes sets up all HTTP handlers.
// GUI API goes under /api/v1 with auth middleware.
// Interactive button actions go under /actions — Mattermost server calls these
// directly and provides user_id in the request body.
func (p *Plugin) initRoutes() {
	// --- GUI API (auth required) ---
	api := p.router.PathPrefix("/api/v1").Subrouter()
	api.Use(p.authMiddleware)

	api.HandleFunc("/resources", p.apiGetResources).Methods("GET")
	api.HandleFunc("/resources", p.apiCreateResource).Methods("POST")
	api.HandleFunc("/resources/{id}", p.apiGetResource).Methods("GET")
	api.HandleFunc("/resources/{id}", p.apiUpdateResource).Methods("PUT")
	api.HandleFunc("/resources/{id}", p.apiDeleteResource).Methods("DELETE")

	api.HandleFunc("/status", p.apiGetAllStatus).Methods("GET")
	api.HandleFunc("/status/{id}", p.apiGetStatus).Methods("GET")

	api.HandleFunc("/resources/{id}/book", p.apiBookResource).Methods("POST")
	api.HandleFunc("/resources/{id}/release", p.apiReleaseResource).Methods("POST")
	api.HandleFunc("/resources/{id}/extend", p.apiExtendResource).Methods("POST")

	api.HandleFunc("/resources/{id}/queue", p.apiJoinQueue).Methods("POST")
	api.HandleFunc("/resources/{id}/queue", p.apiLeaveQueue).Methods("DELETE")

	api.HandleFunc("/resources/{id}/subscribe", p.apiSubscribe).Methods("POST")
	api.HandleFunc("/resources/{id}/unsubscribe", p.apiUnsubscribe).Methods("POST")

	api.HandleFunc("/resources/{id}/history", p.apiGetHistory).Methods("GET")
	api.HandleFunc("/presets", p.apiGetPresets).Methods("GET")

	// --- Interactive button actions (NO auth middleware) ---
	// Mattermost server calls these with PostActionIntegrationRequest in body.
	// Integration URL in buttons: /plugins/com.scientia.resource-queue/actions/book
	// Mattermost strips /plugins/com.scientia.resource-queue → plugin sees /actions/book
	p.router.HandleFunc("/actions/book", p.actionBook).Methods("POST")
	p.router.HandleFunc("/actions/queue", p.actionQueue).Methods("POST")
}

// --- middleware ---

func (p *Plugin) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mattermost-User-ID") == "" {
			httpErr(w, 401, "Unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- helpers ---

func httpJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// actionURL returns the integration URL for interactive buttons.
func actionURL(action string) string {
	return "/plugins/" + pluginID + "/actions/" + action
}

// --- status builder ---

func (p *Plugin) buildStatus(res *Resource, currentUserID string) ResourceStatus {
	booking, _ := p.store.GetBooking(res.ID)
	entries, _ := p.store.GetQueueEntries(res.ID)
	subs, _ := p.store.GetSubscribers(res.ID)

	var bv *BookingView
	if booking != nil {
		bv = &BookingView{Booking: *booking, Username: p.username(booking.UserID)}
	}

	qv := make([]QueueView, 0, len(entries))
	for _, e := range entries {
		qv = append(qv, QueueView{QueueEntry: e, Username: p.username(e.UserID)})
	}

	isSub := false
	for _, uid := range subs {
		if uid == currentUserID {
			isSub = true
			break
		}
	}
	inQ := false
	for _, e := range entries {
		if e.UserID == currentUserID {
			inQ = true
			break
		}
	}

	return ResourceStatus{
		Resource:     *res,
		Booking:      bv,
		Queue:        qv,
		Subscribers:  len(subs),
		IsSubscribed: isSub,
		IsHolder:     booking != nil && booking.UserID == currentUserID,
		InQueue:      inQ,
	}
}

// ===========================
// GUI API handlers
// ===========================

// --- Resources CRUD ---

func (p *Plugin) apiGetResources(w http.ResponseWriter, r *http.Request) {
	res, err := p.store.GetAllResources()
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	httpJSON(w, res)
}

func (p *Plugin) apiGetResource(w http.ResponseWriter, r *http.Request) {
	res, err := p.store.GetResource(mux.Vars(r)["id"])
	if err != nil || res == nil {
		httpErr(w, 404, "not found")
		return
	}
	httpJSON(w, res)
}

func (p *Plugin) apiCreateResource(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	if !p.isAdmin(uid) {
		httpErr(w, 403, "admin only")
		return
	}
	var res Resource
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&res); err != nil {
		httpErr(w, 400, "bad json")
		return
	}
	res.ID = model.NewId()[:8]
	res.Name = truncate(strings.TrimSpace(res.Name), maxNameLen)
	res.IP = truncate(strings.TrimSpace(res.IP), maxIPLen)
	res.Description = truncate(strings.TrimSpace(res.Description), maxDescLen)
	res.CreatedAt = time.Now()
	res.CreatedBy = uid
	if res.Name == "" {
		httpErr(w, 400, "name required")
		return
	}
	// Sanitize variables
	if res.Variables != nil {
		clean := make(map[string]string, len(res.Variables))
		for k, v := range res.Variables {
			k = truncate(strings.TrimSpace(k), maxVarKeyLen)
			v = truncate(strings.TrimSpace(v), maxVarValLen)
			if k != "" {
				clean[k] = v
			}
		}
		res.Variables = clean
	}
	if err := p.store.SaveResource(&res); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	httpJSON(w, res)
}

func (p *Plugin) apiUpdateResource(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	if !p.isAdmin(uid) {
		httpErr(w, 403, "admin only")
		return
	}
	id := mux.Vars(r)["id"]
	existing, err := p.store.GetResource(id)
	if err != nil || existing == nil {
		httpErr(w, 404, "not found")
		return
	}
	var upd Resource
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&upd); err != nil {
		httpErr(w, 400, "bad json")
		return
	}
	existing.Name = truncate(strings.TrimSpace(upd.Name), maxNameLen)
	existing.IP = truncate(strings.TrimSpace(upd.IP), maxIPLen)
	existing.Icon = truncate(strings.TrimSpace(upd.Icon), 10)
	existing.Description = truncate(strings.TrimSpace(upd.Description), maxDescLen)
	if upd.Variables != nil {
		clean := make(map[string]string, len(upd.Variables))
		for k, v := range upd.Variables {
			k = truncate(strings.TrimSpace(k), maxVarKeyLen)
			v = truncate(strings.TrimSpace(v), maxVarValLen)
			if k != "" {
				clean[k] = v
			}
		}
		existing.Variables = clean
	}
	if existing.Name == "" {
		httpErr(w, 400, "name required")
		return
	}
	if err := p.store.SaveResource(existing); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	httpJSON(w, existing)
}

func (p *Plugin) apiDeleteResource(w http.ResponseWriter, r *http.Request) {
	if !p.isAdmin(r.Header.Get("Mattermost-User-ID")) {
		httpErr(w, 403, "admin only")
		return
	}
	if err := p.store.DeleteResource(mux.Vars(r)["id"]); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	httpJSON(w, map[string]string{"status": "ok"})
}

// --- Status ---

func (p *Plugin) apiGetAllStatus(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	resources, err := p.store.GetAllResources()
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	statuses := make([]ResourceStatus, 0, len(resources))
	for _, res := range resources {
		statuses = append(statuses, p.buildStatus(res, uid))
	}
	httpJSON(w, StatusResponse{UserID: uid, IsAdmin: p.isAdmin(uid), Statuses: statuses})
}

func (p *Plugin) apiGetStatus(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	res, err := p.store.GetResource(mux.Vars(r)["id"])
	if err != nil || res == nil {
		httpErr(w, 404, "not found")
		return
	}
	httpJSON(w, p.buildStatus(res, uid))
}

// --- Booking ---

func (p *Plugin) apiBookResource(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		httpErr(w, 404, "not found")
		return
	}
	existing, _ := p.store.GetBooking(id)
	if existing != nil {
		httpErr(w, 409, "resource busy")
		return
	}

	var req struct {
		Minutes int    `json:"minutes"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil || req.Minutes <= 0 {
		httpErr(w, 400, "invalid minutes")
		return
	}
	maxMin := p.cfgMaxBookingHours() * 60
	if req.Minutes > maxMin {
		httpErr(w, 400, fmt.Sprintf("max %d hours", p.cfgMaxBookingHours()))
		return
	}

	b := &Booking{
		ResourceID: id, UserID: uid,
		Purpose:   truncate(req.Purpose, maxPurposeLen),
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(req.Minutes) * time.Minute),
	}
	if err := p.store.SaveBooking(b); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	p.store.RemoveFromQueue(id, uid)
	p.notifySubscribers(id, fmt.Sprintf("🔒 **%s** занят @%s на %s",
		res.Name, p.username(uid), formatDuration(time.Duration(req.Minutes)*time.Minute)), uid)
	httpJSON(w, b)
}

func (p *Plugin) apiReleaseResource(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		httpErr(w, 404, "not found")
		return
	}
	booking, _ := p.store.GetBooking(id)
	if booking == nil {
		httpErr(w, 400, "not booked")
		return
	}
	if booking.UserID != uid && !p.isAdmin(uid) {
		httpErr(w, 403, "not holder")
		return
	}
	p.store.AddHistory(HistoryEntry{
		UserID: booking.UserID, ResourceID: id, Purpose: booking.Purpose,
		StartedAt: booking.StartedAt, EndedAt: time.Now(),
	})
	p.store.DeleteBooking(id)
	p.notifySubscribers(id, fmt.Sprintf("🔓 **%s** освобождён", res.Name), "")
	p.processQueue(id, res.Name)
	httpJSON(w, map[string]string{"status": "released"})
}

func (p *Plugin) apiExtendResource(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	booking, _ := p.store.GetBooking(id)
	if booking == nil {
		httpErr(w, 400, "not booked")
		return
	}
	if booking.UserID != uid {
		httpErr(w, 403, "not holder")
		return
	}

	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req); err != nil || req.Minutes <= 0 {
		httpErr(w, 400, "invalid minutes")
		return
	}
	newExpiry := booking.ExpiresAt.Add(time.Duration(req.Minutes) * time.Minute)
	maxMin := p.cfgMaxBookingHours() * 60
	if int(newExpiry.Sub(booking.StartedAt).Minutes()) > maxMin {
		httpErr(w, 400, fmt.Sprintf("total exceeds max %d hours", p.cfgMaxBookingHours()))
		return
	}
	booking.ExpiresAt = newExpiry
	booking.NotifiedSoon = false
	if err := p.store.SaveBooking(booking); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	httpJSON(w, booking)
}

// --- Queue ---

func (p *Plugin) apiJoinQueue(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	id := mux.Vars(r)["id"]

	res, err := p.store.GetResource(id)
	if err != nil || res == nil {
		httpErr(w, 404, "not found")
		return
	}
	booking, _ := p.store.GetBooking(id)
	if booking != nil && booking.UserID == uid {
		httpErr(w, 400, "you already hold this resource")
		return
	}

	var req struct {
		Minutes int    `json:"minutes"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		httpErr(w, 400, "bad json")
		return
	}
	if req.Minutes <= 0 {
		req.Minutes = 60
	}

	pos, err := p.store.AddToQueue(id, QueueEntry{
		UserID: uid, DesiredDuration: time.Duration(req.Minutes) * time.Minute,
		Purpose: truncate(req.Purpose, maxPurposeLen), QueuedAt: time.Now(),
	})
	if err != nil {
		httpErr(w, 400, err.Error())
		return
	}

	// Notify current holder that someone queued
	if booking != nil && !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**", p.username(uid), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}
	httpJSON(w, map[string]interface{}{"position": pos})
}

func (p *Plugin) apiLeaveQueue(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	p.store.RemoveFromQueue(mux.Vars(r)["id"], uid)
	httpJSON(w, map[string]string{"status": "ok"})
}

// --- Subscriptions ---

func (p *Plugin) apiSubscribe(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	if err := p.store.Subscribe(mux.Vars(r)["id"], uid); err != nil {
		httpErr(w, 400, err.Error())
		return
	}
	httpJSON(w, map[string]string{"status": "subscribed"})
}

func (p *Plugin) apiUnsubscribe(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("Mattermost-User-ID")
	p.store.Unsubscribe(mux.Vars(r)["id"], uid)
	httpJSON(w, map[string]string{"status": "ok"})
}

// --- History ---

func (p *Plugin) apiGetHistory(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	entries, err := p.store.GetHistory(id, 50)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	type histView struct {
		HistoryEntry
		Username string `json:"username"`
	}
	views := make([]histView, 0, len(entries))
	for _, e := range entries {
		views = append(views, histView{HistoryEntry: e, Username: p.username(e.UserID)})
	}
	httpJSON(w, views)
}

func (p *Plugin) apiGetPresets(w http.ResponseWriter, r *http.Request) {
	httpJSON(w, DefaultPresets)
}

// ===========================
// Interactive button actions
// ===========================
// Mattermost POSTs PostActionIntegrationRequest here when user clicks a button.
// Response must be PostActionIntegrationResponse.

func (p *Plugin) actionBook(w http.ResponseWriter, r *http.Request) {
	var req model.PostActionIntegrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		p.API.LogError("actionBook: decode", "err", err.Error())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{
			EphemeralText: "Ошибка запроса",
		})
		return
	}

	p.API.LogDebug("actionBook called",
		"user_id", req.UserId,
		"context", fmt.Sprintf("%v", req.Context))

	uid := req.UserId
	resourceID, _ := req.Context["resource_id"].(string)
	minutesF, _ := req.Context["minutes"].(float64)
	minutes := int(minutesF)

	resp := func(text string) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{EphemeralText: text})
	}

	if uid == "" || resourceID == "" || minutes <= 0 {
		resp("Ошибка: неверные параметры")
		return
	}

	res, err := p.store.GetResource(resourceID)
	if err != nil || res == nil {
		resp("Ресурс не найден")
		return
	}

	existing, _ := p.store.GetBooking(resourceID)
	if existing != nil {
		resp(fmt.Sprintf("🔴 **%s** уже занят @%s", res.Name, p.username(existing.UserID)))
		return
	}

	b := &Booking{
		ResourceID: resourceID, UserID: uid,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(minutes) * time.Minute),
	}
	if err := p.store.SaveBooking(b); err != nil {
		resp("Ошибка: " + err.Error())
		return
	}
	p.store.RemoveFromQueue(resourceID, uid)
	p.notifySubscribers(resourceID, fmt.Sprintf("🔒 **%s** занят @%s на %s",
		res.Name, p.username(uid), formatDuration(time.Duration(minutes)*time.Minute)), uid)

	resp(fmt.Sprintf("✅ **%s** забронирован на %dм (до %s)",
		res.Name, minutes, b.ExpiresAt.Format("15:04")))
}

func (p *Plugin) actionQueue(w http.ResponseWriter, r *http.Request) {
	var req model.PostActionIntegrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{EphemeralText: "Ошибка запроса"})
		return
	}

	uid := req.UserId
	resourceID, _ := req.Context["resource_id"].(string)
	minutesF, _ := req.Context["minutes"].(float64)
	minutes := int(minutesF)
	if minutes <= 0 {
		minutes = 60
	}

	resp := func(text string) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{EphemeralText: text})
	}

	if uid == "" || resourceID == "" {
		resp("Ошибка: неверные параметры")
		return
	}

	res, err := p.store.GetResource(resourceID)
	if err != nil || res == nil {
		resp("Ресурс не найден")
		return
	}

	booking, _ := p.store.GetBooking(resourceID)
	if booking == nil {
		resp(fmt.Sprintf("**%s** свободен — используйте `/rq book %s 1h`", res.Name, res.Name))
		return
	}
	if booking.UserID == uid {
		resp("Вы уже занимаете этот ресурс")
		return
	}

	pos, err := p.store.AddToQueue(resourceID, QueueEntry{
		UserID: uid, DesiredDuration: time.Duration(minutes) * time.Minute, QueuedAt: time.Now(),
	})
	if err != nil {
		resp("Ошибка: " + err.Error())
		return
	}

	if !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**", p.username(uid), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}

	resp(fmt.Sprintf("✅ Вы в очереди на **%s** (позиция: %d)", res.Name, pos))
}
