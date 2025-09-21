package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"vpn-bot/internal/bot"
	"vpn-bot/internal/config"
	"vpn-bot/internal/panel"
	"vpn-bot/internal/panel/auth"
	"vpn-bot/internal/scheduler"
	"vpn-bot/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := storage.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := storage.New(db)
	auth.Configure(cfg.PanelURL, cfg.PanelUser, cfg.PanelPass)

	sessionCookie, err := auth.LoginAndGetSession()
	if err != nil {
		log.Fatalf("panel login: %v", err)
	}

	panelClient := panel.New(cfg.PanelURL, sessionCookie)


	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("new bot: %v", err)
	}

	b := bot.New(api, store, panelClient, cfg.AdminIDs)

	sched := scheduler.New()
	if err := sched.ScheduleDailyNotifications(b); err != nil {
		log.Fatalf("schedule notifications: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := b.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("bot stopped: %v", err)
	}
}
