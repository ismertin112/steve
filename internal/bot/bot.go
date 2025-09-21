package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"vpn-bot/internal/panel"
	"vpn-bot/internal/storage"
)

type Bot struct {
	api             *tgbotapi.BotAPI
	store           *storage.Storage
	panel           *panel.Client
	admins          map[int64]struct{}
	awaitingComment map[int64]int
	mu              sync.Mutex
}

func New(api *tgbotapi.BotAPI, store *storage.Storage, panel *panel.Client, adminIDs []int64) *Bot {
	admins := make(map[int64]struct{})
	for _, id := range adminIDs {
		admins[id] = struct{}{}
	}
	return &Bot{
		api:             api,
		store:           store,
		panel:           panel,
		admins:          admins,
		awaitingComment: make(map[int64]int),
	}
}

func (b *Bot) Run(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := b.api.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message != nil {
		msg := update.Message
		switch {
		case msg.IsCommand():
			b.handleCommand(ctx, msg)
		case msg.Photo != nil:
			b.handlePhoto(ctx, msg)
		default:
			b.handleText(ctx, msg)
		}
	}

	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update.CallbackQuery)
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		b.handleStart(ctx, msg)
	case "getkey":
		b.handleGetKey(ctx, msg)
	case "status":
		b.handleStatus(ctx, msg)
	case "help":
		b.handleHelp(msg.Chat.ID)
	default:
		b.reply(msg.Chat.ID, "Неизвестная команда. Используйте /help")
	}
}

func (b *Bot) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	username := msg.From.UserName
	if username == "" {
		username = msg.From.FirstName
	}
	user, err := b.store.UpsertUser(ctx, msg.From.ID, username)
	if err != nil {
		log.Printf("upsert user: %v", err)
		b.reply(msg.Chat.ID, "Не удалось зарегистрироваться. Попробуйте позже")
		return
	}
	text := fmt.Sprintf("Вы зарегистрированы! Ваш ID в системе: %d", user.ID)
	b.reply(msg.Chat.ID, text)
}

func (b *Bot) handleGetKey(ctx context.Context, msg *tgbotapi.Message) {
	user, err := b.store.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil || user == nil {
		b.reply(msg.Chat.ID, "Сначала выполните /start")
		return
	}
	key, err := b.panel.AddClient(ctx, user.ID)
	if err != nil {
		log.Printf("panel add client: %v", err)
		b.reply(msg.Chat.ID, "Не удалось создать ключ. Попробуйте позже")
		return
	}
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := b.store.UpdateUserKey(ctx, user.ID, key, expires); err != nil {
		log.Printf("update user key: %v", err)
	}
	b.reply(msg.Chat.ID, fmt.Sprintf("Ваш новый ключ: %s\nДействителен до %s", key, expires.Format("02.01.2006")))
}

func (b *Bot) handleStatus(ctx context.Context, msg *tgbotapi.Message) {
	user, err := b.store.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil || user == nil || !user.KeyID.Valid {
		b.reply(msg.Chat.ID, "Ключ не найден. Запросите новый через /getkey")
		return
	}

	expires, err := b.panel.GetClientStatus(ctx, user.KeyID.String)
	if err != nil {
		log.Printf("panel get status: %v", err)
		b.reply(msg.Chat.ID, "Не удалось получить статус. Попробуйте позже")
		return
	}
	b.reply(msg.Chat.ID, fmt.Sprintf("Ключ активен до %s", expires.Format("02.01.2006")))
}

func (b *Bot) handleHelp(chatID int64) {
	helpText := "Инструкция по установке Xray/VLESS:\n" +
		"iOS: используйте приложение Shadowrocket или Streisand.\n" +
		"Android: V2rayNG или Nekobox.\n" +
		"Windows: V2RayN.\n" +
		"Linux/macOS: Xray-core через терминал."
	b.reply(chatID, helpText)
}

func (b *Bot) handlePhoto(ctx context.Context, msg *tgbotapi.Message) {
	user, err := b.store.GetUserByTelegramID(ctx, msg.From.ID)
	if err != nil || user == nil {
		b.reply(msg.Chat.ID, "Сначала выполните /start")
		return
	}
	if len(msg.Photo) == 0 {
		return
	}
	photo := msg.Photo[len(msg.Photo)-1]
	payment, err := b.store.CreatePayment(ctx, user.ID, photo.FileID)
	if err != nil {
		log.Printf("create payment: %v", err)
		b.reply(msg.Chat.ID, "Не удалось сохранить оплату")
		return
	}

	caption := fmt.Sprintf("Новый платеж от @%s (ID %d)", msg.From.UserName, user.ID)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", fmt.Sprintf("confirm:%d", payment.ID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отклонить", fmt.Sprintf("reject:%d", payment.ID)),
		},
	)

	for adminID := range b.admins {
		photoMsg := tgbotapi.NewPhoto(adminID, tgbotapi.FileID(photo.FileID))
		photoMsg.Caption = caption
		photoMsg.ReplyMarkup = keyboard
		if _, err := b.api.Send(photoMsg); err != nil {
			log.Printf("send admin photo: %v", err)
		}
	}

	b.reply(msg.Chat.ID, "Платеж отправлен на проверку")
}

func (b *Bot) handleText(ctx context.Context, msg *tgbotapi.Message) {
	if !b.isAdmin(msg.From.ID) {
		return
	}

	b.mu.Lock()
	paymentID, waiting := b.awaitingComment[msg.From.ID]
	if waiting {
		delete(b.awaitingComment, msg.From.ID)
	}
	b.mu.Unlock()

	if !waiting {
		return
	}

	comment := msg.Text
	if err := b.store.UpdatePaymentStatus(ctx, paymentID, "rejected", &comment); err != nil {
		log.Printf("update payment status: %v", err)
	}

	payment, err := b.store.GetPayment(ctx, paymentID)
	if err != nil {
		log.Printf("get payment: %v", err)
		return
	}

	user, err := b.store.GetUserByID(ctx, payment.UserID)
	if err != nil {
		log.Printf("get user by id: %v", err)
		return
	}

	b.reply(user.TelegramID, fmt.Sprintf("Оплата отклонена: %s", comment))
	b.reply(msg.Chat.ID, "Комментарий отправлен пользователю")
}

func (b *Bot) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	if !b.isAdmin(callback.From.ID) {
		return
	}
	data := callback.Data
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		return
	}
	action := parts[0]
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	switch action {
	case "confirm":
		b.confirmPayment(ctx, callback, id)
	case "reject":
		b.requestRejectReason(callback, id)
	}

	_, _ = b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func (b *Bot) confirmPayment(ctx context.Context, callback *tgbotapi.CallbackQuery, paymentID int) {
	payment, err := b.store.GetPayment(ctx, paymentID)
	if err != nil {
		log.Printf("get payment: %v", err)
		return
	}

	user, err := b.store.GetUserByID(ctx, payment.UserID)
	if err != nil {
		log.Printf("get user: %v", err)
		return
	}

	if !user.KeyID.Valid {
		b.reply(user.TelegramID, "У вас нет активного ключа. Запросите /getkey")
		return
	}

	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := b.panel.UpdateClient(ctx, user.KeyID.String, 30); err != nil {
		log.Printf("panel update: %v", err)
		b.reply(user.TelegramID, "Не удалось продлить подписку. Свяжитесь с админом")
		return
	}

	if err := b.store.UpdateUserKey(ctx, user.ID, user.KeyID.String, expires); err != nil {
		log.Printf("update user key: %v", err)
	}

	if err := b.store.UpdatePaymentStatus(ctx, paymentID, "confirmed", nil); err != nil {
		log.Printf("update payment status: %v", err)
	}

	b.reply(user.TelegramID, fmt.Sprintf("Оплата подтверждена! Новый срок: %s", expires.Format("02.01.2006")))
	b.editCallback(callback, "Оплата подтверждена")
}

func (b *Bot) requestRejectReason(callback *tgbotapi.CallbackQuery, paymentID int) {
	b.mu.Lock()
	b.awaitingComment[callback.From.ID] = paymentID
	b.mu.Unlock()
	b.editCallback(callback, "Отправьте причину отказа сообщением")
}

func (b *Bot) editCallback(callback *tgbotapi.CallbackQuery, text string) {
	msg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("edit message: %v", err)
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send message: %v", err)
	}
}

func (b *Bot) isAdmin(id int64) bool {
	_, ok := b.admins[id]
	return ok
}

func (b *Bot) NotifyRenewal(ctx context.Context, when time.Time) error {
	from := when
	to := when.Add(24 * time.Hour)
	users, err := b.store.ListUsersExpiringBetween(ctx, from, to)
	if err != nil {
		return err
	}
	for _, user := range users {
		if !user.ExpiresAt.Valid {
			continue
		}
		text := fmt.Sprintf("Напоминание об оплате. Срок действия до %s", user.ExpiresAt.Time.Format("02.01.2006"))
		b.reply(user.TelegramID, text)
	}
	return nil
}
