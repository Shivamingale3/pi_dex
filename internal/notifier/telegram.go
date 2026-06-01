package notifier

import (
	"PI_DEX/internal/core"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TelegramNotifier struct {
	Token  string
	ChatID string
	Client *http.Client
}

func NewTelegramNotifier(
	token string,
	chatID string,
) *TelegramNotifier {

	return &TelegramNotifier{
		Token:  token,
		ChatID: chatID,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramNotifier) Send(
	event core.Event,
) error {

	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/sendMessage",
		t.Token,
	)

	text := fmt.Sprintf(
		"🚨 PiDex Alert\n\n"+
			"Severity: %s\n"+
			"Source: %s\n"+
			"Host: %s\n\n"+
			"%s\n\n"+
			"%s\n\n"+
			"%s",

		event.Severity,
		event.Source,
		event.Hostname,
		event.Title,
		event.Message,
		event.Timestamp.Format(time.RFC3339),
	)

	payload := map[string]any{
		"chat_id": t.ChatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := t.Client.Post(
		url,
		"application/json",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"telegram returned status %d",
			resp.StatusCode,
		)
	}

	return nil
}
