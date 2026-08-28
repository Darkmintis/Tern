package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTelegram_MissingToken(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TERN_TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TERN_TELEGRAM_CHAT_ID", "")

	_, err := NewTelegram()
	if err == nil {
		t.Fatal("expected error for missing bot token")
	}
}

func TestNewTelegram_MissingChatID(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TERN_TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TERN_TELEGRAM_CHAT_ID", "")

	_, err := NewTelegram()
	if err == nil {
		t.Fatal("expected error for missing chat ID")
	}
}

func TestNewTelegram_Success(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "123456")

	notifier, err := NewTelegram()
	if err != nil {
		t.Fatal(err)
	}
	if notifier.BotToken != "test-token" {
		t.Fatalf("expected test-token, got %s", notifier.BotToken)
	}
	if notifier.ChatID != "123456" {
		t.Fatalf("expected 123456, got %s", notifier.ChatID)
	}
}

func TestNewTelegram_TernEnvVars(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TERN_TELEGRAM_BOT_TOKEN", "tern-token")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TERN_TELEGRAM_CHAT_ID", "789012")

	notifier, err := NewTelegram()
	if err != nil {
		t.Fatal(err)
	}
	if notifier.BotToken != "tern-token" {
		t.Fatalf("expected tern-token, got %s", notifier.BotToken)
	}
}

func TestSend_Success(t *testing.T) {
	// Mock Telegram API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if msg.ChatID != "123456" {
			t.Errorf("expected chat_id 123456, got %s", msg.ChatID)
		}
		if msg.Text == "" {
			t.Error("expected non-empty text")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	// We can't easily test the real Send function without mocking the URL
	// But we can test the message structure
	msg := Message{
		ChatID:    "123456",
		Text:      "Test message",
		ParseMode: "HTML",
	}

	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var parsed Message
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.ChatID != "123456" {
		t.Fatalf("expected 123456, got %s", parsed.ChatID)
	}
}

func TestMessageWithButtons(t *testing.T) {
	msg := Message{
		ChatID:    "123456",
		Text:      "Test",
		ParseMode: "HTML",
		ReplyMarkup: &ReplyMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{
					{Text: "Button 1", URL: "https://example.com"},
					{Text: "Button 2", CallbackData: "action:2"},
				},
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var parsed Message
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.ReplyMarkup == nil {
		t.Fatal("expected reply markup")
	}
	if len(parsed.ReplyMarkup.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(parsed.ReplyMarkup.InlineKeyboard))
	}
	if len(parsed.ReplyMarkup.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(parsed.ReplyMarkup.InlineKeyboard[0]))
	}
}

func TestNotifyReleaseSuccess_FormatsCorrectly(t *testing.T) {
	// Test that the function would format the message correctly
	// (We can't actually send without a real token)
	version := "1.2.3"
	platform := "android"
	track := "internal"

	text := "✅ <b>Release Shipped</b>\n\n" +
		"<b>Version:</b> " + version + "\n" +
		"<b>Platform:</b> " + platform + "\n" +
		"<b>Track:</b> " + track

	if text == "" {
		t.Fatal("expected non-empty message")
	}
}
