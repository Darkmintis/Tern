package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/sendMessage") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Errorf("invalid json: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if msg.ChatID != "123456" {
			t.Errorf("expected chat_id 123456, got %s", msg.ChatID)
		}
		if msg.Text != "hello" {
			t.Errorf("expected text hello, got %s", msg.Text)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "123456",
		BaseURL:  server.URL,
	}

	err := notifier.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSend_WithButtons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Errorf("invalid json: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if msg.ReplyMarkup == nil {
			t.Error("expected reply_markup")
		}
		if len(msg.ReplyMarkup.InlineKeyboard) != 1 {
			t.Errorf("expected 1 row, got %d", len(msg.ReplyMarkup.InlineKeyboard))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "123456",
		BaseURL:  server.URL,
	}

	buttons := [][]InlineKeyboardButton{
		{{Text: "Promote", URL: "https://example.com"}},
	}
	err := notifier.Send(context.Background(), "test", buttons...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSend_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "forbidden"}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "bad-token",
		ChatID:   "123456",
		BaseURL:  server.URL,
	}

	err := notifier.Send(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestNotifyReleaseSuccess_FormatsCorrectly(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "123456",
		BaseURL:  server.URL,
	}

	// We can't call NotifyReleaseSuccess directly since it creates its own notifier.
	// But we can test the message format by calling Send with the same format.
	text := "✅ <b>Release Shipped</b>\n\n" +
		"<b>Version:</b> 1.2.3\n" +
		"<b>Platform:</b> android\n" +
		"<b>Track:</b> internal"

	buttons := [][]InlineKeyboardButton{
		{
			{Text: "🚀 Promote to Production", URL: "https://telegram.me/tern-bot?start=promote_production"},
			{Text: "⏪ Emergency Rollback", URL: "https://telegram.me/tern-bot?start=rollback"},
		},
	}

	err := notifier.Send(context.Background(), text, buttons...)
	if err != nil {
		t.Fatal(err)
	}

	var msg Message
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Text, "Release Shipped") {
		t.Errorf("missing release text: %s", msg.Text)
	}
	if msg.ReplyMarkup == nil {
		t.Error("expected reply markup")
	}
}

func TestMessageJSON(t *testing.T) {
	msg := Message{
		ChatID:    "123456",
		Text:      "hello",
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
	if parsed.ParseMode != "HTML" {
		t.Fatalf("expected HTML, got %s", parsed.ParseMode)
	}
}
