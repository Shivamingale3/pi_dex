package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

const maxRetries = 3

var severityIcon = map[string]string{
	core.SeverityInfo:     "\u2139\ufe0f",
	core.SeverityWarn:     "\u26a0\ufe0f",
	core.SeverityCritical: "\U0001f6a8",
	core.SeverityRecovered: "\u2705",
}

type TelegramNotifier struct {
	apiURL   string
	chatID   string
	client   *http.Client
	hostname string
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	hostname, _ := os.Hostname()
	return &TelegramNotifier{
		apiURL:   fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
		hostname: hostname,
	}
}

func (n *TelegramNotifier) Send(event core.Event) error {
	text := n.formatMessage(event)
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

func (n *TelegramNotifier) formatMessage(event core.Event) string {
	icon := severityIcon[event.Severity]
	lines := []string{
		fmt.Sprintf("%s <b>%s | %s</b>", icon, event.Severity, html.EscapeString(event.Title)),
		html.EscapeString(event.Message),
		"",
		fmt.Sprintf("  Server    %s", n.hostname),
		fmt.Sprintf("  Source    %s", html.EscapeString(event.Source)),
		fmt.Sprintf("  Time      %s", event.Timestamp.Format("2006-01-02 15:04:05")),
		"",
		fmt.Sprintf("\u2014 PiDex v%s", core.Version),
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
