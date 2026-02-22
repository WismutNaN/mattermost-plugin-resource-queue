package main

type configuration struct {
	EnablePlugin         bool   `json:"EnablePlugin"`
	NotifyBeforeMinutes  string `json:"NotifyBeforeMinutes"`
	MaxBookingHours      string `json:"MaxBookingHours"`
	CheckIntervalSeconds string `json:"CheckIntervalSeconds"`
}

func (p *Plugin) getConfiguration() *configuration {
	cfg := &configuration{
		NotifyBeforeMinutes:  "10",
		MaxBookingHours:      "24",
		CheckIntervalSeconds: "30",
	}
	if err := p.API.LoadPluginConfiguration(cfg); err != nil {
		p.API.LogWarn("Failed to load plugin config, using defaults", "error", err.Error())
	}
	return cfg
}
