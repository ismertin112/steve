package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramToken string
	AdminIDs      []int64
	PanelURL      string
	PanelUser     string
	PanelPass     string
	DBDSN         string
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}

	admins := os.Getenv("ADMIN_IDS")
	if admins != "" {
		parts := strings.Split(admins, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := parseInt64(part)
			if err != nil {
				return nil, fmt.Errorf("invalid admin id %q: %w", part, err)
			}
			cfg.AdminIDs = append(cfg.AdminIDs, id)
		}
	}

	cfg.PanelURL = os.Getenv("PANEL_URL")
	if cfg.PanelURL == "" {
		return nil, fmt.Errorf("PANEL_URL is required")
	}

	cfg.PanelUser = os.Getenv("PANEL_USER")
	if cfg.PanelUser == "" {
		return nil, fmt.Errorf("PANEL_USER is required")
	}

	cfg.PanelPass = os.Getenv("PANEL_PASS")
	if cfg.PanelPass == "" {
		return nil, fmt.Errorf("PANEL_PASS is required")
	}

	cfg.DBDSN = os.Getenv("DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	return cfg, nil
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
