package main

import (
	"fmt"
	"time"
)

type Scheduler struct {
	plugin *Plugin
	stop   chan struct{}
}

func NewScheduler(p *Plugin) *Scheduler {
	return &Scheduler{
		plugin: p,
		stop:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.run()
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) run() {
	interval := time.Duration(s.plugin.getCheckIntervalSeconds()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkBookings()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) checkBookings() {
	ids, err := s.plugin.store.GetResourceList()
	if err != nil {
		return
	}

	notifyBefore := time.Duration(s.plugin.getNotifyBeforeMinutes()) * time.Minute

	for _, id := range ids {
		booking, err := s.plugin.store.GetBookingRaw(id)
		if err != nil || booking == nil {
			continue
		}

		res, _ := s.plugin.store.GetResource(id)
		resName := id
		if res != nil {
			resName = res.Name
		}

		now := time.Now()
		timeLeft := booking.ExpiresAt.Sub(now)

		// Check if expired
		if timeLeft <= 0 {
			// Auto-release
			s.plugin.store.AddHistory(HistoryEntry{
				UserID:     booking.UserID,
				ResourceID: id,
				Purpose:    booking.Purpose,
				StartedAt:  booking.StartedAt,
				EndedAt:    booking.ExpiresAt,
			})
			s.plugin.store.DeleteBooking(id)
			s.plugin.sendDM(booking.UserID, fmt.Sprintf("⏰ Время бронирования **%s** истекло. Ресурс освобождён.", resName))
			s.plugin.notifySubscribers(id, fmt.Sprintf("🔓 **%s** освобождён (время истекло)", resName), "")
			s.plugin.processQueue(id, resName)
			continue
		}

		// Notify soon expiry
		if timeLeft <= notifyBefore && !booking.NotifiedSoon {
			s.plugin.sendDM(booking.UserID, fmt.Sprintf("⚠️ Бронирование **%s** истечёт через %s. Используйте `/rq release %s` чтобы освободить или `/rq extend %s <время>` чтобы продлить.",
				resName, formatTimeLeft(timeLeft), id, id))
			booking.NotifiedSoon = true
			s.plugin.store.SaveBooking(booking)
		}
	}
}
