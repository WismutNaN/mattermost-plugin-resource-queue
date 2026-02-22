package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) sendDM(userID, message string) {
	channel, appErr := p.API.GetDirectChannel(p.botUserID, userID)
	if appErr != nil {
		p.API.LogError("Failed to get DM channel", "user_id", userID, "error", appErr.Error())
		return
	}

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channel.Id,
		Message:   message,
	}
	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("Failed to send DM", "user_id", userID, "error", appErr.Error())
	}
}

func (p *Plugin) notifySubscribers(resourceID, message, excludeUserID string) {
	subs, _ := p.store.GetSubscribers(resourceID)
	for _, uid := range subs {
		if uid != excludeUserID {
			p.sendDM(uid, message)
		}
	}
}

func (p *Plugin) processQueue(resourceID, resourceName string) {
	entry, err := p.store.PopQueue(resourceID)
	if err != nil || entry == nil {
		return
	}

	p.sendDM(entry.UserID, fmt.Sprintf("🎉 **%s** теперь свободен! Вы следующий в очереди. Забронируйте его командой:\n`/rq book %s %s`",
		resourceName, resourceID, formatDuration(entry.DesiredDuration)))
}

func (p *Plugin) getUsername(userID string) string {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return "unknown"
	}
	return user.Username
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dм", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dч", hours)
	}
	return fmt.Sprintf("%dч%dм", hours, mins)
}

func formatTimeLeft(d time.Duration) string {
	if d < 0 {
		return "истекло"
	}
	if d < time.Minute {
		return fmt.Sprintf("%d сек", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d мин", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d ч %d мин", hours, mins)
}
