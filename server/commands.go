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
		AutoComplete:     true,
		AutoCompleteHint: "[list|book|release|extend|queue|leave|subscribe|history|help]",
		AutoCompleteDesc: "Управление общими ресурсами",
	})
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	parts := strings.Fields(args.Command)
	if len(parts) < 2 {
		return p.cmdHelp(), nil
	}
	sub := strings.ToLower(parts[1])
	rest := parts[2:]

	switch sub {
	case "list", "ls", "l":
		return p.cmdList()
	case "status", "st", "s":
		return p.cmdStatus(rest)
	case "book", "b":
		return p.cmdBook(args.UserId, rest)
	case "release", "free", "r":
		return p.cmdRelease(args.UserId, rest)
	case "extend", "e":
		return p.cmdExtend(args.UserId, rest)
	case "queue", "q":
		return p.cmdQueue(args.UserId, rest)
	case "leave":
		return p.cmdLeave(args.UserId, rest)
	case "subscribe", "sub", "watch":
		return p.cmdSubscribe(args.UserId, rest)
	case "unsubscribe", "unsub", "unwatch":
		return p.cmdUnsubscribe(args.UserId, rest)
	case "history", "hist":
		return p.cmdHistory(rest)
	default:
		return p.cmdHelp(), nil
	}
}

func eph(text string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
}

// --- List ---

func (p *Plugin) cmdList() (*model.CommandResponse, *model.AppError) {
	resources, err := p.store.GetAllResources()
	if err != nil {
		return eph("Ошибка: " + err.Error()), nil
	}
	if len(resources) == 0 {
		return eph("Ресурсы не настроены. Администратор может добавить их через GUI (кнопка 🖥️)."), nil
	}

	attachments := make([]*model.SlackAttachment, 0, len(resources))
	for _, r := range resources {
		icon := r.Icon
		if icon == "" {
			icon = "🖥️"
		}

		booking, _ := p.store.GetBooking(r.ID)
		entries, _ := p.store.GetQueueEntries(r.ID)

		var line, color string
		if booking != nil {
			left := time.Until(booking.ExpiresAt)
			parts := []string{
				fmt.Sprintf("%s **%s**", icon, r.Name),
			}
			if r.IP != "" {
				parts = append(parts, fmt.Sprintf("`%s`", r.IP))
			}
			parts = append(parts, fmt.Sprintf("🔴 @%s ⏱%s", p.username(booking.UserID), formatTimeLeft(left)))
			if booking.Purpose != "" {
				parts = append(parts, fmt.Sprintf("_%s_", booking.Purpose))
			}
			if len(entries) > 0 {
				parts = append(parts, fmt.Sprintf("👥%d", len(entries)))
			}
			line = strings.Join(parts, " · ")
			color = "#e53935"
		} else {
			parts := []string{fmt.Sprintf("%s **%s**", icon, r.Name)}
			if r.IP != "" {
				parts = append(parts, fmt.Sprintf("`%s`", r.IP))
			}
			parts = append(parts, "🟢 Свободен")
			line = strings.Join(parts, " · ")
			color = "#4caf50"
		}

		var actions []*model.PostAction
		if booking == nil {
			actions = []*model.PostAction{
				{
					Id: "b10_" + r.ID, Name: "⚡10м", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     actionURL("book"),
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 10},
					},
				},
				{
					Id: "b60_" + r.ID, Name: "🔒1ч", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     actionURL("book"),
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 60},
					},
				},
			}
		} else {
			actions = []*model.PostAction{
				{
					Id: "q60_" + r.ID, Name: "📋Очередь 1ч", Type: "button",
					Integration: &model.PostActionIntegration{
						URL:     actionURL("queue"),
						Context: map[string]interface{}{"resource_id": r.ID, "minutes": 60},
					},
				},
			}
		}

		attachments = append(attachments, &model.SlackAttachment{
			Text: line, Color: color, Actions: actions,
		})
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Attachments:  attachments,
	}, nil
}

// --- Status ---

func (p *Plugin) cmdStatus(args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) == 0 {
		resources, err := p.store.GetAllResources()
		if err != nil {
			return eph("Ошибка: " + err.Error()), nil
		}
		var sb strings.Builder
		for _, r := range resources {
			booking, _ := p.store.GetBooking(r.ID)
			icon := r.Icon
			if icon == "" {
				icon = "🖥️"
			}
			if booking != nil {
				left := time.Until(booking.ExpiresAt)
				sb.WriteString(fmt.Sprintf("%s **%s** — 🔴 @%s ⏱%s\n", icon, r.Name, p.username(booking.UserID), formatTimeLeft(left)))
			} else {
				sb.WriteString(fmt.Sprintf("%s **%s** — 🟢 Свободен\n", icon, r.Name))
			}
		}
		return eph(sb.String()), nil
	}

	res, err := p.findResource(strings.Join(args, " "))
	if err != nil {
		return eph(err.Error()), nil
	}

	booking, _ := p.store.GetBooking(res.ID)
	entries, _ := p.store.GetQueueEntries(res.ID)
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
		sb.WriteString(fmt.Sprintf("%s\n", res.Description))
	}
	if booking != nil {
		left := time.Until(booking.ExpiresAt)
		sb.WriteString(fmt.Sprintf("**Статус:** 🔴 Занят @%s (⏱ %s)\n", p.username(booking.UserID), formatTimeLeft(left)))
		if booking.Purpose != "" {
			sb.WriteString(fmt.Sprintf("**Цель:** %s\n", booking.Purpose))
		}
	} else {
		sb.WriteString("**Статус:** 🟢 Свободен\n")
	}
	if len(entries) > 0 {
		sb.WriteString(fmt.Sprintf("**Очередь:** %d\n", len(entries)))
		for i, e := range entries {
			sb.WriteString(fmt.Sprintf("  %d. @%s", i+1, p.username(e.UserID)))
			if e.Purpose != "" {
				sb.WriteString(fmt.Sprintf(" — %s", e.Purpose))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString(fmt.Sprintf("**Подписчики:** %d\n", len(subs)))

	return eph(sb.String()), nil
}

// --- Book ---

func (p *Plugin) cmdBook(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return eph("Использование: `/rq book <имя> <время> [цель]`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	dur, err := parseDuration(args[1])
	if err != nil {
		return eph(err.Error()), nil
	}
	maxMin := p.cfgMaxBookingHours() * 60
	if int(dur.Minutes()) > maxMin {
		return eph(fmt.Sprintf("Максимум %d часов", p.cfgMaxBookingHours())), nil
	}
	existing, _ := p.store.GetBooking(res.ID)
	if existing != nil {
		return eph(fmt.Sprintf("🔴 **%s** занят @%s (⏱ %s)", res.Name, p.username(existing.UserID), formatTimeLeft(time.Until(existing.ExpiresAt)))), nil
	}

	purpose := ""
	if len(args) > 2 {
		purpose = truncate(strings.Join(args[2:], " "), maxPurposeLen)
	}
	b := &Booking{
		ResourceID: res.ID, UserID: userID, Purpose: purpose,
		StartedAt: time.Now(), ExpiresAt: time.Now().Add(dur),
	}
	if err := p.store.SaveBooking(b); err != nil {
		return eph("Ошибка: " + err.Error()), nil
	}
	p.store.RemoveFromQueue(res.ID, userID)
	p.notifySubscribers(res.ID, fmt.Sprintf("🔒 **%s** занят @%s на %s", res.Name, p.username(userID), formatDuration(dur)), userID)
	return eph(fmt.Sprintf("✅ **%s** забронирован на %s (до %s)", res.Name, formatDuration(dur), b.ExpiresAt.Format("15:04"))), nil
}

// --- Release ---

func (p *Plugin) cmdRelease(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return eph("Использование: `/rq release <имя>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	booking, _ := p.store.GetBooking(res.ID)
	if booking == nil {
		return eph("**" + res.Name + "** не забронирован"), nil
	}
	if booking.UserID != userID && !p.isAdmin(userID) {
		return eph("Только текущий пользователь или админ может освободить"), nil
	}
	p.store.AddHistory(HistoryEntry{
		UserID: booking.UserID, ResourceID: res.ID, Purpose: booking.Purpose,
		StartedAt: booking.StartedAt, EndedAt: time.Now(),
	})
	p.store.DeleteBooking(res.ID)
	p.notifySubscribers(res.ID, fmt.Sprintf("🔓 **%s** освобождён", res.Name), "")
	p.processQueue(res.ID, res.Name)
	return eph(fmt.Sprintf("🔓 **%s** освобождён", res.Name)), nil
}

// --- Extend ---

func (p *Plugin) cmdExtend(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return eph("Использование: `/rq extend <имя> <время>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	booking, _ := p.store.GetBooking(res.ID)
	if booking == nil {
		return eph("**" + res.Name + "** не забронирован"), nil
	}
	if booking.UserID != userID {
		return eph("Только текущий пользователь может продлить"), nil
	}
	dur, err := parseDuration(args[1])
	if err != nil {
		return eph(err.Error()), nil
	}
	newExpiry := booking.ExpiresAt.Add(dur)
	maxMin := p.cfgMaxBookingHours() * 60
	if int(newExpiry.Sub(booking.StartedAt).Minutes()) > maxMin {
		return eph(fmt.Sprintf("Суммарно превышает максимум %d часов", p.cfgMaxBookingHours())), nil
	}
	booking.ExpiresAt = newExpiry
	booking.NotifiedSoon = false
	p.store.SaveBooking(booking)
	return eph(fmt.Sprintf("⏳ **%s** продлён на %s (до %s)", res.Name, formatDuration(dur), newExpiry.Format("15:04"))), nil
}

// --- Queue ---

func (p *Plugin) cmdQueue(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 2 {
		return eph("Использование: `/rq queue <имя> <время> [цель]`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	dur, err := parseDuration(args[1])
	if err != nil {
		return eph(err.Error()), nil
	}
	booking, _ := p.store.GetBooking(res.ID)
	if booking != nil && booking.UserID == userID {
		return eph("Вы уже занимаете **" + res.Name + "**"), nil
	}

	purpose := ""
	if len(args) > 2 {
		purpose = truncate(strings.Join(args[2:], " "), maxPurposeLen)
	}
	pos, err := p.store.AddToQueue(res.ID, QueueEntry{
		UserID: userID, DesiredDuration: dur, Purpose: purpose, QueuedAt: time.Now(),
	})
	if err != nil {
		return eph("Ошибка: " + err.Error()), nil
	}
	if booking != nil && !booking.NotifiedQueue {
		p.sendDM(booking.UserID, fmt.Sprintf("👋 @%s встал в очередь на **%s**", p.username(userID), res.Name))
		booking.NotifiedQueue = true
		p.store.SaveBooking(booking)
	}
	return eph(fmt.Sprintf("✅ Вы в очереди на **%s** (позиция: %d)", res.Name, pos)), nil
}

// --- Leave ---

func (p *Plugin) cmdLeave(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return eph("Использование: `/rq leave <имя>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	p.store.RemoveFromQueue(res.ID, userID)
	return eph(fmt.Sprintf("Вы покинули очередь на **%s**", res.Name)), nil
}

// --- Subscribe/Unsubscribe ---

func (p *Plugin) cmdSubscribe(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return eph("Использование: `/rq subscribe <имя>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	if err := p.store.Subscribe(res.ID, userID); err != nil {
		return eph(err.Error()), nil
	}
	return eph(fmt.Sprintf("🔔 Подписка на **%s** оформлена", res.Name)), nil
}

func (p *Plugin) cmdUnsubscribe(userID string, args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return eph("Использование: `/rq unsubscribe <имя>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	p.store.Unsubscribe(res.ID, userID)
	return eph(fmt.Sprintf("🔕 Подписка на **%s** отменена", res.Name)), nil
}

// --- History ---

func (p *Plugin) cmdHistory(args []string) (*model.CommandResponse, *model.AppError) {
	if len(args) < 1 {
		return eph("Использование: `/rq history <имя>`"), nil
	}
	res, err := p.findResource(args[0])
	if err != nil {
		return eph(err.Error()), nil
	}
	entries, _ := p.store.GetHistory(res.ID, 20)
	if len(entries) == 0 {
		return eph("История **" + res.Name + "** пуста"), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Последние сессии — %s\n", res.Name))
	for _, e := range entries {
		dur := e.EndedAt.Sub(e.StartedAt)
		purpose := ""
		if e.Purpose != "" {
			purpose = fmt.Sprintf(" — %s", e.Purpose)
		}
		sb.WriteString(fmt.Sprintf("• @%s · %s · %s%s\n",
			p.username(e.UserID), e.StartedAt.Format("02.01 15:04"), formatDuration(dur), purpose))
	}
	return eph(sb.String()), nil
}

// --- Help ---

func (p *Plugin) cmdHelp() *model.CommandResponse {
	return eph(`### Resource Queue
| Команда | Описание |
|---|---|
| ` + "`/rq list`" + ` | Список ресурсов с кнопками |
| ` + "`/rq status [имя]`" + ` | Подробный статус |
| ` + "`/rq book <имя> <время> [цель]`" + ` | Забронировать |
| ` + "`/rq release <имя>`" + ` | Освободить |
| ` + "`/rq extend <имя> <время>`" + ` | Продлить |
| ` + "`/rq queue <имя> <время> [цель]`" + ` | Встать в очередь |
| ` + "`/rq leave <имя>`" + ` | Покинуть очередь |
| ` + "`/rq subscribe <имя>`" + ` | Подписка на уведомления |
| ` + "`/rq history <имя>`" + ` | История |
**Время:** ` + "`30m` `1h` `2h30m`" + ` или число минут`)
}

// --- Helpers ---

func (p *Plugin) findResource(nameOrID string) (*Resource, error) {
	resources, err := p.store.GetAllResources()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(nameOrID))
	var matches []*Resource
	for _, r := range resources {
		if strings.ToLower(r.ID) == q || strings.ToLower(r.Name) == q {
			return r, nil
		}
		if strings.Contains(strings.ToLower(r.Name), q) || strings.HasPrefix(r.ID, q) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = "`" + m.Name + "`"
		}
		return nil, fmt.Errorf("неоднозначно: %s", strings.Join(names, ", "))
	}
	return nil, fmt.Errorf("ресурс `%s` не найден", nameOrID)
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if mins, err := strconv.Atoi(s); err == nil && mins > 0 {
		return time.Duration(mins) * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("неверный формат: `%s` (примеры: 30m, 1h, 2h30m)", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("время должно быть > 0")
	}
	return d, nil
}
