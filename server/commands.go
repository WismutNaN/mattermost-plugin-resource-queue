package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) registerCommands() error {
	return p.API.RegisterCommand(&model.Command{
		Trigger:          "rq",
		DisplayName:      "Resource Queue",
		Description:      "Manage shared resources, bookings and queues",
		AutoComplete:     true,
		AutoCompleteDesc: "Resource Queue commands",
		AutoCompleteHint: "[list|book|release|extend|queue|leave|subscribe|unsubscribe|status|history|help]",
	})
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	parts := strings.Fields(args.Command)
	if len(parts) < 2 {
		return p.cmdHelp(), nil
	}

	sub := parts[1]
	cmdArgs := parts[2:]

	switch sub {
	case "list", "ls":
		return p.cmdList()
	case "status", "st":
		return p.cmdStatus(cmdArgs)
	case "book", "b":
		return p.cmdBook(args.UserId, cmdArgs)
	case "release", "free", "r":
		return p.cmdRelease(args.UserId, cmdArgs)
	case "extend", "e":
		return p.cmdExtend(args.UserId, cmdArgs)
	case "queue", "q":
		return p.cmdQueue(args.UserId, cmdArgs)
	case "leave", "l":
		return p.cmdLeave(args.UserId, cmdArgs)
	case "subscribe", "sub":
		return p.cmdSubscribe(args.UserId, cmdArgs)
	case "unsubscribe", "unsub":
		return p.cmdUnsubscribe(args.UserId, cmdArgs)
	case "history", "hist":
		return p.cmdHistory(cmdArgs)
	case "help", "h":
		return p.cmdHelp(), nil
	default:
		return p.cmdHelp(), nil
	}
}

func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}

func (p *Plugin) cmdHelp() *model.CommandResponse {
	text := `### Resource Queue — Команды
| Команда | Описание |
|---|---|
| ` + "`/rq list`" + ` | Список всех ресурсов |
| ` + "`/rq status [name]`" + ` | Статус ресурса или всех ресурсов |
| ` + "`/rq book <name> <время> [цель]`" + ` | Забронировать ресурс (30m, 2h, 4h30m) |
| ` + "`/rq release <name>`" + ` | Освободить ресурс |
| ` + "`/rq extend <name> <время>`" + ` | Продлить бронирование |
| ` + "`/rq queue <name> <время> [цель]`" + ` | Встать в очередь |
| ` + "`/rq leave <name>`" + ` | Покинуть очередь |
| ` + "`/rq subscribe <name>`" + ` | Подписаться на уведомления |
| ` + "`/rq unsubscribe <name>`" + ` | Отписаться от уведомлений |
| ` + "`/rq history <name>`" + ` | История использования |

**Время:** ` + "`30m`" + `, ` + "`1h`" + `, ` + "`2h30m`" + `, ` + "`4h`" + `, и т.д.
**Имя:** Имя ресурса или его ID (часть).`
	return ephemeral(text)
}

func (p *Plugin) findResource(nameOrID string) (*Resource, error) {
	resources, err := p.store.GetAllResources()
	if err != nil {
		return nil, err
	}

	nameOrID = strings.ToLower(nameOrID)
	var matches []*Resource

	for _, r := range resources {
		if strings.ToLower(r.ID) == nameOrID || strings.ToLower(r.Name) == nameOrID {
			return r, nil
		}
		if strings.Contains(strings.ToLower(r.Name), nameOrID) || strings.HasPrefix(r.ID, nameOrID) {
			matches = append(matches, r)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = fmt.Sprintf("`%s`", m.Name)
		}
		return nil, fmt.Errorf("неоднозначное имя, найдено: %s", strings.Join(names, ", "))
	}
	return nil, fmt.Errorf("ресурс `%s` не найден", nameOrID)
}

func parseDuration(s string) (time.Duration, error) {
	// Support formats: 30m, 1h, 2h30m, 90 (minutes)
	s = strings.TrimSpace(strings.ToLower(s))

	// If just a number, treat as minutes
	if mins, err := strconv.Atoi(s); err == nil {
		return time.Duration(mins) * time.Minute, nil
	}

	// Try Go duration parser
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("неверный формат времени: `%s`. Примеры: 30m, 1h, 2h30m", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("время должно быть положительным")
	}
	return d, nil
}

func (p *Plugin) cmdList() (*model.CommandResponse, *model.AppError) {
	resources, err := p.store.GetAllResources()
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}
	if len(resources) == 0 {
		return ephemeral("Ресурсы не настроены. Администратор может добавить их через GUI плагина."), nil
	}

	pluginURL := p.getPluginURL()
	attachments := make([]*model.SlackAttachment, 0, len(resources))

	for _, r := range resources {
		icon := r.Icon
		if icon == "" {
			icon = "🖥️"
		}

		booking, _ := p.store.GetBooking(r.ID)
		queue, _ := p.store.GetQueue(r.ID)

		var line, color string

		if booking != nil {
			timeLeft := time.Until(booking.ExpiresAt)
			purpose := ""
			if booking.Purpose != "" {
				purpose = fmt.Sprintf(" · _%s_", booking.Purpose)
			}
			queueInfo := ""
			if len(queue.Entries) > 0 {
				queueInfo = fmt.Sprintf(" · 👥%d", len(queue.Entries))
			}
			line = fmt.Sprintf("%s **%s** `%s` · 🔴 @%s ⏱%s%s%s",
				icon, r.Name, r.IP, p.getUsername(booking.UserID), formatTimeLeft(timeLeft), purpose, queueInfo)
			color = "#e53935"
		} else {
			line = fmt.Sprintf("%s **%s** `%s` · 🟢 Свободен", icon, r.Name, r.IP)
			color = "#4caf50"
		}

		actions := []*model.PostAction{}
		if booking == nil {
			actions = append(actions,
				&model.PostAction{
					Id: "b10_" + r.ID, Name: "⚡10м", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     pluginURL + "/action/book",
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 10},
					},
				},
				&model.PostAction{
					Id: "b60_" + r.ID, Name: "🔒1ч", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     pluginURL + "/action/book",
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 60},
					},
				},
			)
		} else {
			actions = append(actions,
				&model.PostAction{
					Id: "q_" + r.ID, Name: "📋Очередь 1ч", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     pluginURL + "/action/queue",
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 60},
					},
				},
			)
		}

		attachments = append(attachments, &model.SlackAttachment{
			Text:    line,
			Color:   color,
			Actions: actions,
		})
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Attachments:  attachments,
	}, nil
}

func (p *Plugin) cmdStatus(args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) == 0 {
		// Show all
		resources, err := p.store.GetAllResources()
		if err != nil {
			return ephemeral("Ошибка: " + err.Error()), nil
		}
		if len(resources) == 0 {
			return ephemeral("Ресурсы не настроены."), nil
		}

		var sb strings.Builder
		sb.WriteString("### Статус ресурсов\n")
		for _, r := range resources {
			booking, _ := p.store.GetBooking(r.ID)
			queue, _ := p.store.GetQueue(r.ID)
			icon := r.Icon
			if icon == "" {
				icon = "🖥️"
			}

			if booking != nil {
				timeLeft := time.Until(booking.ExpiresAt)
				purpose := ""
				if booking.Purpose != "" {
					purpose = fmt.Sprintf(" — _%s_", booking.Purpose)
				}
				sb.WriteString(fmt.Sprintf("🔴 %s **%s** — @%s (осталось %s)%s",
					icon, r.Name, p.getUsername(booking.UserID), formatTimeLeft(timeLeft), purpose))
			} else {
				sb.WriteString(fmt.Sprintf("🟢 %s **%s** — свободен", icon, r.Name))
			}

			if len(queue.Entries) > 0 {
				sb.WriteString(fmt.Sprintf(" | очередь: %d", len(queue.Entries)))
			}
			sb.WriteString("\n")
		}
		return ephemeral(sb.String()), nil
	}

	// Single resource
	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	booking, _ := p.store.GetBooking(res.ID)
	queue, _ := p.store.GetQueue(res.ID)
	subs, _ := p.store.GetSubscribers(res.ID)

	var sb strings.Builder
	icon := res.Icon
	if icon == "" {
		icon = "🖥️"
	}
	sb.WriteString(fmt.Sprintf("### %s %s\n", icon, res.Name))
	if res.IP != "" {
		sb.WriteString(fmt.Sprintf("**IP:** `%s`\n", res.IP))
	}
	if res.Description != "" {
		sb.WriteString(fmt.Sprintf("**Описание:** %s\n", res.Description))
	}
	if len(res.Variables) > 0 {
		sb.WriteString("**Переменные:**\n")
		for k, v := range res.Variables {
			sb.WriteString(fmt.Sprintf("- `%s` = `%s`\n", k, v))
		}
	}

	sb.WriteString("\n**Статус:** ")
	if booking != nil {
		timeLeft := time.Until(booking.ExpiresAt)
		sb.WriteString(fmt.Sprintf("🔴 Занят @%s (осталось %s)", p.getUsername(booking.UserID), formatTimeLeft(timeLeft)))
		if booking.Purpose != "" {
			sb.WriteString(fmt.Sprintf("\n**Цель:** %s", booking.Purpose))
		}
	} else {
		sb.WriteString("🟢 Свободен")
	}

	if len(queue.Entries) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n**Очередь (%d):**\n", len(queue.Entries)))
		for i, e := range queue.Entries {
			purpose := ""
			if e.Purpose != "" {
				purpose = fmt.Sprintf(" — %s", e.Purpose)
			}
			sb.WriteString(fmt.Sprintf("%d. @%s (%s)%s\n", i+1, p.getUsername(e.UserID), formatDuration(e.DesiredDuration), purpose))
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Подписчиков:** %d", len(subs)))

	return ephemeral(sb.String()), nil
}

func (p *Plugin) cmdBook(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return ephemeral("Использование: `/rq book <name> <время> [цель]`"), nil
	}

	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	d, err := parseDuration(args[1])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	maxMinutes := p.getMaxBookingHours() * 60
	if int(d.Minutes()) > maxMinutes {
		return ephemeral(fmt.Sprintf("Максимальная длительность бронирования: %d ч", p.getMaxBookingHours())), nil
	}

	existing, _ := p.store.GetBooking(res.ID)
	if existing != nil {
		return ephemeral(fmt.Sprintf("🔴 **%s** уже занят @%s (осталось %s). Используйте `/rq queue %s %s` чтобы встать в очередь.",
			res.Name, p.getUsername(existing.UserID), formatTimeLeft(time.Until(existing.ExpiresAt)), args[0], args[1])), nil
	}

	purpose := ""
	if len(args) > 2 {
		purpose = strings.Join(args[2:], " ")
	}

	booking := &Booking{
		ResourceID: res.ID,
		UserID:     userID,
		Purpose:    purpose,
		StartedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(d),
	}

	if err := p.store.SaveBooking(booking); err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	p.store.RemoveFromQueue(res.ID, userID)
	p.notifySubscribers(res.ID, fmt.Sprintf("🔒 **%s** занят @%s на %s",
		res.Name, p.getUsername(userID), formatDuration(d)), userID)

	return ephemeral(fmt.Sprintf("✅ Вы забронировали **%s** на %s (до %s)",
		res.Name, formatDuration(d), booking.ExpiresAt.Format("15:04"))), nil
}

func (p *Plugin) cmdRelease(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return ephemeral("Использование: `/rq release <name>`"), nil
	}

	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	booking, _ := p.store.GetBooking(res.ID)
	if booking == nil {
		return ephemeral(fmt.Sprintf("**%s** не занят.", res.Name)), nil
	}

	if booking.UserID != userID && !p.isAdmin(userID) {
		return ephemeral("Только текущий пользователь или админ может освободить ресурс."), nil
	}

	p.store.AddHistory(HistoryEntry{
		UserID:     booking.UserID,
		ResourceID: res.ID,
		Purpose:    booking.Purpose,
		StartedAt:  booking.StartedAt,
		EndedAt:    time.Now(),
	})
	p.store.DeleteBooking(res.ID)
	p.notifySubscribers(res.ID, fmt.Sprintf("🔓 **%s** освобождён", res.Name), "")
	p.processQueue(res.ID, res.Name)

	return ephemeral(fmt.Sprintf("✅ **%s** освобождён.", res.Name)), nil
}

func (p *Plugin) cmdExtend(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return ephemeral("Использование: `/rq extend <name> <время>`"), nil
	}

	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	booking, _ := p.store.GetBooking(res.ID)
	if booking == nil {
		return ephemeral(fmt.Sprintf("**%s** не занят.", res.Name)), nil
	}
	if booking.UserID != userID {
		return ephemeral("Только текущий пользователь может продлить."), nil
	}

	d, err := parseDuration(args[1])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	booking.ExpiresAt = booking.ExpiresAt.Add(d)
	booking.NotifiedSoon = false

	maxMinutes := p.getMaxBookingHours() * 60
	totalMinutes := int(booking.ExpiresAt.Sub(booking.StartedAt).Minutes())
	if totalMinutes > maxMinutes {
		return ephemeral(fmt.Sprintf("Общая длительность превышает максимум (%d ч)", p.getMaxBookingHours())), nil
	}

	p.store.SaveBooking(booking)
	return ephemeral(fmt.Sprintf("✅ Бронирование **%s** продлено на %s (до %s)",
		res.Name, formatDuration(d), booking.ExpiresAt.Format("15:04"))), nil
}

func (p *Plugin) cmdQueue(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return ephemeral("Использование: `/rq queue <name> <время> [цель]`"), nil
	}

	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	d, err := parseDuration(args[1])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	// If resource is free, suggest booking instead
	booking, _ := p.store.GetBooking(res.ID)
	if booking == nil {
		return ephemeral(fmt.Sprintf("**%s** свободен! Используйте `/rq book %s %s` для бронирования.", res.Name, args[0], args[1])), nil
	}

	if booking.UserID == userID {
		return ephemeral("Вы уже занимаете этот ресурс."), nil
	}

	purpose := ""
	if len(args) > 2 {
		purpose = strings.Join(args[2:], " ")
	}

	entry := QueueEntry{
		UserID:          userID,
		DesiredDuration: d,
		Purpose:         purpose,
		QueuedAt:        time.Now(),
	}

	pos, err := p.store.AddToQueue(res.ID, entry)
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	if !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**", p.getUsername(userID), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}

	return ephemeral(fmt.Sprintf("✅ Вы в очереди на **%s** (позиция: %d). Вы получите уведомление когда ресурс освободится.",
		res.Name, pos)), nil
}

func (p *Plugin) cmdLeave(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return ephemeral("Использование: `/rq leave <name>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}
	p.store.RemoveFromQueue(res.ID, userID)
	return ephemeral(fmt.Sprintf("✅ Вы покинули очередь на **%s**.", res.Name)), nil
}

func (p *Plugin) cmdSubscribe(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return ephemeral("Использование: `/rq subscribe <name>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}
	if err := p.store.Subscribe(res.ID, userID); err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}
	return ephemeral(fmt.Sprintf("✅ Вы подписаны на уведомления о **%s**.", res.Name)), nil
}

func (p *Plugin) cmdUnsubscribe(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return ephemeral("Использование: `/rq unsubscribe <name>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}
	p.store.Unsubscribe(res.ID, userID)
	return ephemeral(fmt.Sprintf("✅ Вы отписаны от **%s**.", res.Name)), nil
}

func (p *Plugin) cmdHistory(args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return ephemeral("Использование: `/rq history <name>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	history, err := p.store.GetHistory(res.ID)
	if err != nil {
		return ephemeral("Ошибка: " + err.Error()), nil
	}

	if len(history.Entries) == 0 {
		return ephemeral(fmt.Sprintf("История **%s** пуста.", res.Name)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### История %s (последние %d)\n", res.Name, len(history.Entries)))
	sb.WriteString("| Пользователь | Начало | Длительность | Цель |\n|---|---|---|---|\n")

	limit := 20
	if len(history.Entries) < limit {
		limit = len(history.Entries)
	}
	for _, e := range history.Entries[:limit] {
		dur := e.EndedAt.Sub(e.StartedAt)
		purpose := e.Purpose
		if purpose == "" {
			purpose = "—"
		}
		sb.WriteString(fmt.Sprintf("| @%s | %s | %s | %s |\n",
			p.getUsername(e.UserID), e.StartedAt.Format("02.01 15:04"), formatDuration(dur), purpose))
	}
	return ephemeral(sb.String()), nil
}
