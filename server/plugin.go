package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin
	configurationLock sync.RWMutex
	store             *Store
	router            *mux.Router
	scheduler         *Scheduler
	botUserID         string
}

func (p *Plugin) OnActivate() error {
	p.store = NewStore(p.API)

	// Create or get bot account
	botUserID, err := p.ensureBot()
	if err != nil {
		p.API.LogError("Failed to ensure bot", "error", err.Error())
		return err
	}
	p.botUserID = botUserID

	// Register commands
	if err := p.registerCommands(); err != nil {
		return err
	}

	// Setup HTTP router
	p.router = mux.NewRouter()
	p.setupAPI()

	// Start scheduler
	p.scheduler = NewScheduler(p)
	p.scheduler.Start()

	return nil
}

func (p *Plugin) ensureBot() (string, error) {
	// Check if we already stored the bot ID
	if data, appErr := p.API.KVGet("bot_user_id"); appErr == nil && len(data) > 0 {
		botUserID := string(data)
		// Verify bot still exists
		if user, uErr := p.API.GetUser(botUserID); uErr == nil && user != nil {
			return botUserID, nil
		}
	}

	// Try to find existing bot user by username
	user, appErr := p.API.GetUserByUsername("resource-queue")
	if appErr == nil && user != nil {
		p.API.KVSet("bot_user_id", []byte(user.Id))
		return user.Id, nil
	}

	// Create new bot
	newBot, createErr := p.API.CreateBot(&model.Bot{
		Username:    "resource-queue",
		DisplayName: "Resource Queue",
		Description: "Resource Queue Bot",
	})
	if createErr != nil {
		return "", fmt.Errorf("create bot: %v", createErr)
	}
	p.API.KVSet("bot_user_id", []byte(newBot.UserId))
	return newBot.UserId, nil
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

func (p *Plugin) getNotifyBeforeMinutes() int {
	cfg := p.getConfiguration()
	m, err := strconv.Atoi(cfg.NotifyBeforeMinutes)
	if err != nil || m <= 0 {
		return 10
	}
	return m
}

func (p *Plugin) getMaxBookingHours() int {
	cfg := p.getConfiguration()
	h, err := strconv.Atoi(cfg.MaxBookingHours)
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

func (p *Plugin) getCheckIntervalSeconds() int {
	cfg := p.getConfiguration()
	s, err := strconv.Atoi(cfg.CheckIntervalSeconds)
	if err != nil || s <= 0 {
		return 30
	}
	return s
}

func (p *Plugin) getPluginURL() string {
	siteURL := ""
	cfg := p.API.GetConfig()
	if cfg != nil && cfg.ServiceSettings.SiteURL != nil {
		siteURL = *cfg.ServiceSettings.SiteURL
	}
	return siteURL + "/plugins/com.scientia.resource-queue"
}

// main is required for plugin
func main() {
	plugin.ClientMain(&Plugin{})
}
