package notifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

const (
	maxRetries = 3
	// Telegram rejects messages longer than 4096 characters.
	maxMessageLen = 4096
	// Longest Retry-After (from HTTP 429) we are willing to sleep inside Send.
	maxRetryAfter = 30 * time.Second
)

var severityIcon = map[string]string{
	core.SeverityInfo:      "ℹ️",
	core.SeverityWarn:      "⚠️",
	core.SeverityCritical:  "\U0001f6a8",
	core.SeverityRecovered: "✅",
}

// PermanentError is returned when Telegram rejected the message itself
// (e.g. HTTP 400). Retrying the same message later will not help, so callers
// should not queue it.
type PermanentError struct {
	Status int
	Body   string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("telegram rejected message: HTTP %d: %s", e.Status, e.Body)
}

// IsPermanent reports whether err means the message can never be delivered.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}

type TelegramNotifier struct {
	apiURL   string
	chatID   string
	client   *http.Client
	hostname string
	sleep    func(time.Duration)
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	hostname, _ := os.Hostname()
	return &TelegramNotifier{
		apiURL:   fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
		hostname: hostname,
		sleep:    time.Sleep,
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
			n.sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			log.Printf("Sent %s notification", event.EventType)
			return nil
		case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge:
			return &PermanentError{Status: resp.StatusCode, Body: describe(respBody)}
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, describe(respBody))
		log.Printf("Telegram send failed (attempt %d/%d): %v", attempt+1, maxRetries, lastErr)
		wait := time.Duration(1<<attempt) * time.Second
		if resp.StatusCode == http.StatusTooManyRequests {
			if ra := retryAfter(respBody); ra > wait && ra <= maxRetryAfter {
				wait = ra
			}
		}
		n.sleep(wait)
	}

	return fmt.Errorf("telegram send failed after %d retries: %w", maxRetries, lastErr)
}

type apiResponse struct {
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func describe(body []byte) string {
	var r apiResponse
	if json.Unmarshal(body, &r) == nil && r.Description != "" {
		return r.Description
	}
	return strings.TrimSpace(string(body))
}

func retryAfter(body []byte) time.Duration {
	var r apiResponse
	if json.Unmarshal(body, &r) != nil {
		return 0
	}
	return time.Duration(r.Parameters.RetryAfter) * time.Second
}

const timeLayout = "2006-01-02 15:04:05"

func (n *TelegramNotifier) formatMessage(event core.Event) string {
	icon := severityIcon[event.Severity]
	head := []string{
		fmt.Sprintf("%s <b>%s | %s</b>", icon, event.Severity, html.EscapeString(event.Title)),
	}
	tail := []string{
		"",
		fmt.Sprintf("  Server    %s", html.EscapeString(n.hostname)),
		fmt.Sprintf("  Source    %s", html.EscapeString(event.Source)),
		fmt.Sprintf("  Time      %s", event.Timestamp.Format(timeLayout)),
		"",
		fmt.Sprintf("— PiDex v%s", core.Version),
	}
	if event.Delayed {
		tail = append([]string{"", fmt.Sprintf("<i>Delayed: originally at %s</i>", event.Timestamp.Format(timeLayout))}, tail...)
	}

	fixed := len(strings.Join(head, "\n")) + len(strings.Join(tail, "\n")) + 2
	budget := maxMessageLen - fixed

	msg := truncateEscaped(html.EscapeString(event.Message), budget)
	budget -= len(msg)

	body := []string{msg}
	if event.Details != "" {
		const pre, endPre = "\n<pre>", "</pre>"
		if room := budget - len(pre) - len(endPre); room > 32 {
			body = append(body, pre[1:]+truncateEscaped(html.EscapeString(event.Details), room)+endPre)
		}
	}

	return strings.Join(append(append(head, body...), tail...), "\n")
}

// truncateEscaped shortens HTML-escaped text to at most max bytes without
// splitting an entity (&amp;) or a UTF-8 sequence. Byte length is an upper
// bound on Telegram's character count, so the result always fits.
func truncateEscaped(s string, max int) string {
	if len(s) <= max {
		return s
	}
	const ellipsis = "…"
	if max <= len(ellipsis) {
		return ""
	}
	cut := max - len(ellipsis)
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	if amp := strings.LastIndexByte(s[:cut], '&'); amp >= 0 && !strings.Contains(s[amp:cut], ";") {
		cut = amp
	}
	return s[:cut] + ellipsis
}

func isRuneStart(b byte) bool {
	return b&0xC0 != 0x80
}
