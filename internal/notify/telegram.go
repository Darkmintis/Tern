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
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
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
func (t *TelegramNotifier) Send(ctx context.Context, text string) error {
	msg := Message{
		ChatID:    t.ChatID,
		Text:      text,
		ParseMode: "HTML",
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

// NotifyRelease sends a release notification.
func NotifyRelease(ctx context.Context, version, platform, track, status string) error {
	t, err := NewTelegram()
	if err != nil {
		return err
	}

	emoji := "✅"
	if status != "ok" {
		emoji = "❌"
	}

	text := fmt.Sprintf(
		"%s <b>Release %s</b>\n\n"+
			"📦 Platform: %s\n"+
			"🎯 Track: %s\n"+
			"📊 Status: %s\n"+
			"🕐 Time: %s",
		emoji, version, platform, track, status,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	return t.Send(ctx, text)
}
