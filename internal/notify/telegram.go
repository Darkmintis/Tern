package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// TelegramNotifier sends messages to Telegram.
type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

// Message represents a Telegram message.
type Message struct {
	ChatID      string       `json:"chat_id"`
	Text        string       `json:"text"`
	ParseMode   string       `json:"parse_mode,omitempty"`
	ReplyMarkup *ReplyMarkup `json:"reply_markup,omitempty"`
}

// ReplyMarkup for inline keyboards.
type ReplyMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton is a Telegram inline button.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

// NewTelegram creates a new Telegram notifier from env.
func NewTelegram() (*TelegramNotifier, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		botToken = os.Getenv("TERN_TELEGRAM_BOT_TOKEN")
	}
	if botToken == "" {
		return nil, ternerrors.NewHint(ternerrors.ClassConfig, "Telegram bot token not set",
			"set TELEGRAM_BOT_TOKEN env or TERN_TELEGRAM_BOT_TOKEN")
	}

	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if chatID == "" {
		chatID = os.Getenv("TERN_TELEGRAM_CHAT_ID")
	}
	if chatID == "" {
		return nil, ternerrors.NewHint(ternerrors.ClassConfig, "Telegram chat ID not set",
			"set TELEGRAM_CHAT_ID env or TERN_TELEGRAM_CHAT_ID")
	}

	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
	}, nil
}

// Send sends a message to the configured Telegram chat.
func (t *TelegramNotifier) Send(ctx context.Context, text string, buttons ...[]InlineKeyboardButton) error {
	msg := Message{
		ChatID:    t.ChatID,
		Text:      text,
		ParseMode: "HTML",
	}

	if len(buttons) > 0 {
		msg.ReplyMarkup = &ReplyMarkup{
			InlineKeyboard: buttons,
		}
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassConfig, "marshal telegram message", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassConfig, "create telegram request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassConfig, "send telegram message", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return ternerrors.NewHint(ternerrors.ClassConfig, "telegram API error",
			fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody)))
	}

	return nil
}

// NotifyReleaseSuccess sends success with emergency actions.
func NotifyReleaseSuccess(ctx context.Context, version, platform, track string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"✅ <b>Release Shipped</b>\n\n"+
			"<b>Version:</b> %s\n"+
			"<b>Platform:</b> %s\n"+
			"<b>Track:</b> %s\n"+
			"<b>Time:</b> %s\n\n"+
			"<code>tern promote %s production</code>\n"+
			"<code>tern rollback</code>",
		version, platform, track,
		time.Now().Format("15:04:05"),
		track,
	)

	buttons := [][]InlineKeyboardButton{
		{
			{Text: "🚀 Promote to Production", URL: "https://telegram.me/tern-bot?start=promote_production"},
			{Text: "⏪ Emergency Rollback", URL: "https://telegram.me/tern-bot?start=rollback"},
		},
		{
			{Text: "📊 Check Status", URL: "https://telegram.me/tern-bot?start=status"},
		},
	}

	return t.Send(ctx, text, buttons...)
}

// NotifyReleaseFailure sends failure with retry and rollback.
func NotifyReleaseFailure(ctx context.Context, version, platform, track, errMsg string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	// Truncate error message
	if len(errMsg) > 200 {
		errMsg = errMsg[:200] + "..."
	}

	text := fmt.Sprintf(
		"🚨 <b>RELEASE FAILED</b>\n\n"+
			"<b>Version:</b> %s\n"+
			"<b>Platform:</b> %s\n"+
			"<b>Track:</b> %s\n"+
			"<b>Error:</b>\n<code>%s</code>\n\n"+
			"<b>Quick fix:</b>\n"+
			"<code>tern release_%s --force</code>\n"+
			"<code>tern rollback</code>",
		version, platform, track, errMsg,
		track,
	)

	buttons := [][]InlineKeyboardButton{
		{
			{Text: "🔄 Retry Release", URL: "https://telegram.me/tern-bot?start=retry"},
			{Text: "⏪ Rollback Now", URL: "https://telegram.me/tern-bot?start=rollback"},
		},
		{
			{Text: "📜 View Full Logs", URL: "https://telegram.me/tern-bot?start=logs"},
		},
	}

	return t.Send(ctx, text, buttons...)
}

// NotifyBuildStatus sends build progress.
func NotifyBuildStatus(ctx context.Context, status, version, platform string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	var emoji string
	var extra string
	switch status {
	case "started":
		emoji = "🔨"
		extra = "Building..."
	case "completed":
		emoji = "✅"
		extra = "Ready to upload"
	case "failed":
		emoji = "❌"
		extra = "Check logs"
	default:
		emoji = "📦"
		extra = status
	}

	text := fmt.Sprintf(
		"%s <b>Build %s</b>\n\n"+
			"<b>Version:</b> %s\n"+
			"<b>Platform:</b> %s\n"+
			"<b>Status:</b> %s",
		emoji, status, version, platform, extra,
	)

	return t.Send(ctx, text)
}

// NotifyEmergency sends critical alerts that need immediate attention.
func NotifyEmergency(ctx context.Context, message string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"🚨 <b>EMERGENCY</b>\n\n"+
			"%s\n\n"+
			"<b>Time:</b> %s\n"+
			"<b>Immediate actions:</b>\n"+
			"<code>tern rollback</code>\n"+
			"<code>tern status</code>",
		message,
		time.Now().Format("15:04:05"),
	)

	buttons := [][]InlineKeyboardButton{
		{
			{Text: "⏪ ROLLBACK NOW", URL: "https://telegram.me/tern-bot?start=emergency_rollback"},
		},
	}

	return t.Send(ctx, text, buttons...)
}
