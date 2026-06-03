package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"time"

	"github.com/leadows/pi_dex/internal/core"
)

const maxRetries = 3

var severityIcon = map[string]string{
	core.SeverityInfo:     "\u2139\ufe0f",
	core.SeverityWarn:     "\u26a0\ufe0f",
	core.SeverityCritical: "\U0001f6a8",
	core.SeverityRecovered: "\u2705",
}

type TelegramNotifier struct {
	apiURL  string
	chatID  string
	client  *http.Client
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		apiURL: fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		chatID: chatID,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *TelegramNotifier) Send(event core.Event) error {
	text := formatMessage(event)
	payload := map[string]any{
		"chat_id":                  n.chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := range maxRetries {
		resp, err := n.client.Post(n.apiURL, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			log.Printf("Telegram send failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Sent %s notification", event.EventType)
			return nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		log.Printf("Telegram send failed (attempt %d/%d): HTTP %d", attempt+1, maxRetries, resp.StatusCode)
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}

	return fmt.Errorf("telegram send failed after %d retries: %w", maxRetries, lastErr)
}

func formatMessage(event core.Event) string {
	icon := severityIcon[event.Severity]
	safeMsg := html.EscapeString(event.Message)
	lines := []string{
		fmt.Sprintf("%s <b>%s</b>", icon, html.EscapeString(event.Title)),
		fmt.Sprintf("<code>%s</code>", safeMsg),
		fmt.Sprintf("\U0001f4c5 %s", event.Timestamp.Format("2006-01-02 15:04:05")),
		fmt.Sprintf("\U0001f3e0 %s", html.EscapeString(event.Source)),
	}
	var result string
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
