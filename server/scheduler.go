package main

import (
	"fmt"
	"time"
)

type Scheduler struct {
	plugin *Plugin
	stop   chan struct{}
	done   chan struct{}
}

func NewScheduler(p *Plugin) *Scheduler {
	return &Scheduler{
		plugin: p,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.run()
}

func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done // wait for goroutine to exit
}

func (s *Scheduler) run() {
	defer close(s.done)

	interval := time.Duration(s.plugin.cfgCheckSeconds()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) tick() {
	ids, err := s.plugin.store.getResourceIDs()
	if err != nil {
		return
	}

	notifyBefore := time.Duration(s.plugin.cfgNotifyMinutes()) * time.Minute

	for _, id := range ids {
		// Use Raw to see expired bookings before cleanup
		booking, err := s.plugin.store.GetBookingRaw(id)
		if err != nil || booking == nil {
			continue
		}

		res, _ := s.plugin.store.GetResource(id)
		name := id
		if res != nil {
			name = res.Name
		}

		left := time.Until(booking.ExpiresAt)

		if left <= 0 {
			// Expired — auto-release
			s.plugin.store.AddHistory(HistoryEntry{
				UserID:     booking.UserID,
				ResourceID: id,
				Purpose:    booking.Purpose,
				StartedAt:  booking.StartedAt,
				EndedAt:    booking.ExpiresAt,
			})
			s.plugin.store.DeleteBooking(id)
			s.plugin.sendDM(booking.UserID,
				fmt.Sprintf("⏰ Время бронирования **%s** истекло. Ресурс освобождён.", name))
			s.plugin.notifySubscribers(id,
				fmt.Sprintf("🔓 **%s** освобождён (время истекло)", name), "")
			s.plugin.processQueue(id, name)
			continue
		}

		// Warn before expiry
		if left <= notifyBefore && !booking.NotifiedSoon {
			s.plugin.sendDM(booking.UserID,
				fmt.Sprintf("⚠️ Бронирование **%s** истечёт через %s. `/rq extend %s <время>` чтобы продлить.",
					name, formatTimeLeft(left), name))
			booking.NotifiedSoon = true
			s.plugin.store.SaveBooking(booking)
		}
	}
}
