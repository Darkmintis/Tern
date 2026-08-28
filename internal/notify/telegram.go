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
	ChatID      string        `json:"chat_id"`
	Text        string        `json:"text"`
	ParseMode   string        `json:"parse_mode,omitempty"`
	ReplyMarkup *ReplyMarkup  `json:"reply_markup,omitempty"`
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return ternerrors.NewHint(ternerrors.ClassConfig, "telegram API error",
			fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody)))
	}

	return nil
}

// NotifyReleaseSuccess sends a success notification with action buttons.
func NotifyReleaseSuccess(ctx context.Context, version, platform, track, projectRoot string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"✅ <b>Release Successful</b>\n\n"+
			"📦 Version: %s\n"+
			"📱 Platform: %s\n"+
			"🎯 Track: %s\n"+
			"🕐 Time: %s",
		version, platform, track,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// Action buttons
	buttons := [][]InlineKeyboardButton{
		{
			{Text: "📊 View Status", CallbackData: "tern:status"},
			{Text: "📜 View History", CallbackData: "tern:history"},
		},
		{
			{Text: "🔄 Promote to Beta", CallbackData: fmt.Sprintf("tern:promote:%s:beta", track)},
			{Text: "🚀 Promote to Prod", CallbackData: fmt.Sprintf("tern:promote:%s:production", track)},
		},
		{
			{Text: "⏪ Rollback", CallbackData: "tern:rollback"},
		},
	}

	return t.Send(ctx, text, buttons...)
}

// NotifyReleaseFailure sends a failure notification with retry button.
func NotifyReleaseFailure(ctx context.Context, version, platform, track, errMsg string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"❌ <b>Release Failed</b>\n\n"+
			"📦 Version: %s\n"+
			"📱 Platform: %s\n"+
			"🎯 Track: %s\n"+
			"⚠️ Error: %s\n\n"+
			"<i>Check CI logs for details.</i>",
		version, platform, track, errMsg,
	)

	buttons := [][]InlineKeyboardButton{
		{
			{Text: "🔄 Retry Release", CallbackData: "tern:retry"},
			{Text: "📜 View Logs", CallbackData: "tern:logs"},
		},
	}

	return t.Send(ctx, text, buttons...)
}

// NotifyBuildStatus sends build progress notifications.
func NotifyBuildStatus(ctx context.Context, status, version, platform string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	var emoji string
	switch status {
	case "started":
		emoji = "🔨"
	case "completed":
		emoji = "✅"
	case "failed":
		emoji = "❌"
	default:
		emoji = "📦"
	}

	text := fmt.Sprintf(
		"%s <b>Build %s</b>\n\n"+
			"📦 Version: %s\n"+
			"📱 Platform: %s",
		emoji, status, version, platform,
	)

	return t.Send(ctx, text)
}
