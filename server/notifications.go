package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) sendDM(userID, text string) {
	channel, err := p.API.GetDirectChannel(userID, p.botUserID)
	if err != nil {
		p.API.LogWarn("sendDM: GetDirectChannel", "user", userID, "err", err.Error())
		return
	}
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channel.Id,
		Message:   text,
	}
	if _, err := p.API.CreatePost(post); err != nil {
		p.API.LogWarn("sendDM: CreatePost", "user", userID, "err", err.Error())
	}
}

func (p *Plugin) notifySubscribers(resourceID, text, excludeUserID string) {
	subs, _ := p.store.GetSubscribers(resourceID)
	for _, uid := range subs {
		if uid != excludeUserID {
			p.sendDM(uid, text)
		}
	}
}

func (p *Plugin) processQueue(resourceID, resourceName string) {
	entry, err := p.store.PopQueue(resourceID)
	if err != nil || entry == nil {
		return
	}
	p.sendDM(entry.UserID, fmt.Sprintf(
		"🎉 **%s** свободен! Вы следующий в очереди.\nИспользуйте `/rq book %s %s` чтобы занять.",
		resourceName, resourceName, formatDuration(entry.DesiredDuration)))
}

// --- formatting ---

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

func formatTimeLeft(d time.Duration) string {
	if d <= 0 {
		return "истекает"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dч%02dм", h, m)
	}
	return fmt.Sprintf("%dм", m)
}
