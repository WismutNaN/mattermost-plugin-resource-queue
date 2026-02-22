package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin
	mu        sync.RWMutex
	store     *Store
	router    *mux.Router
	scheduler *Scheduler
	botUserID string
}

func (p *Plugin) OnActivate() error {
	p.store = NewStore(p.API)

	botID, err := p.ensureBot()
	if err != nil {
		return fmt.Errorf("ensure bot: %w", err)
	}
	p.botUserID = botID

	if err := p.registerCommands(); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	p.router = mux.NewRouter()
	p.initRoutes()

	p.scheduler = NewScheduler(p)
	p.scheduler.Start()

	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.scheduler != nil {
		p.scheduler.Stop()
	}
	return nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) ensureBot() (string, error) {
	if data, appErr := p.API.KVGet(keyBotUserID); appErr == nil && len(data) > 0 {
		uid := string(data)
		if u, e := p.API.GetUser(uid); e == nil && u != nil {
			return uid, nil
		}
	}
	if u, e := p.API.GetUserByUsername(botUsername); e == nil && u != nil {
		p.API.KVSet(keyBotUserID, []byte(u.Id))
		return u.Id, nil
	}
	bot, err := p.API.CreateBot(&model.Bot{
		Username:    botUsername,
		DisplayName: "Resource Queue",
		Description: "Manages shared resource bookings",
	})
	if err != nil {
		return "", fmt.Errorf("CreateBot: %v", err)
	}
	p.API.KVSet(keyBotUserID, []byte(bot.UserId))
	return bot.UserId, nil
}

// --- config helpers ---

type configuration struct {
	EnablePlugin         bool   `json:"EnablePlugin"`
	NotifyBeforeMinutes  string `json:"NotifyBeforeMinutes"`
	MaxBookingHours      string `json:"MaxBookingHours"`
	CheckIntervalSeconds string `json:"CheckIntervalSeconds"`
}

func (p *Plugin) getConfig() *configuration {
	cfg := &configuration{NotifyBeforeMinutes: "10", MaxBookingHours: "24", CheckIntervalSeconds: "30"}
	_ = p.API.LoadPluginConfiguration(cfg)
	return cfg
}

func (p *Plugin) cfgNotifyMinutes() int  { v, _ := strconv.Atoi(p.getConfig().NotifyBeforeMinutes); if v <= 0 { return 10 }; return v }
func (p *Plugin) cfgMaxBookingHours() int { v, _ := strconv.Atoi(p.getConfig().MaxBookingHours); if v <= 0 { return 24 }; return v }
func (p *Plugin) cfgCheckSeconds() int   { v, _ := strconv.Atoi(p.getConfig().CheckIntervalSeconds); if v <= 0 { return 30 }; return v }

// --- user helpers ---

func (p *Plugin) isAdmin(userID string) bool {
	u, err := p.API.GetUser(userID)
	if err != nil {
		return false
	}
	return strings.Contains(u.Roles, "system_admin")
}

func (p *Plugin) username(userID string) string {
	u, err := p.API.GetUser(userID)
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func main() {
	plugin.ClientMain(&Plugin{})
}
