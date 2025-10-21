package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// TelegramService handles sending messages to Telegram
type TelegramService struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramService creates a new Telegram service
func NewTelegramService() *TelegramService {
	return &TelegramService{
		botToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		chatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// telegramMessage represents a Telegram API message
type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// ContactMessageInfo contains additional information about the contact form submission
type ContactMessageInfo struct {
	IPAddress       string
	UserAgent       string
	Referrer        string
	UserID          *int    // Optional: set if user is authenticated
	UserEmail       *string // Optional: user's registered email if authenticated
	IsAuthenticated bool    // Whether the user was logged in when submitting
}

// SendContactMessage sends a contact form message to Telegram
func (s *TelegramService) SendContactMessage(name, email, message string, info *ContactMessageInfo) error {
	if s.botToken == "" || s.chatID == "" {
		return fmt.Errorf("telegram bot token or chat ID not configured")
	}

	// Format the message with additional metadata
	textBuilder := fmt.Sprintf(
		"🆕 <b>New Contact Form Submission</b>\n\n"+
			"<b>Name:</b> %s\n"+
			"<b>Email:</b> %s\n"+
			"<b>Message:</b>\n%s\n\n"+
			"━━━━━━━━━━━━━━━━━━━━\n"+
			"<b>📊 Submission Details</b>\n"+
			"<b>Time:</b> %s\n"+
			"<b>IP Address:</b> %s\n"+
			"<b>User Agent:</b> %s\n"+
			"<b>Referrer:</b> %s",
		escapeHTML(name),
		escapeHTML(email),
		escapeHTML(message),
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		escapeHTML(info.IPAddress),
		escapeHTML(truncate(info.UserAgent, 100)),
		escapeHTML(getOrDefault(info.Referrer, "Direct")),
	)

	// Add authenticated user info if available
	if info.IsAuthenticated && info.UserID != nil {
		authInfo := fmt.Sprintf(
			"\n\n<b>👤 Authenticated User</b>\n"+
				"<b>User ID:</b> %d",
			*info.UserID,
		)
		if info.UserEmail != nil {
			authInfo += fmt.Sprintf("\n<b>Registered Email:</b> %s", escapeHTML(*info.UserEmail))
		}
		textBuilder += authInfo
	}

	text := textBuilder

	// Prepare the request
	payload := telegramMessage{
		ChatID:    s.chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %w", err)
	}

	// Send to Telegram
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// escapeHTML escapes HTML special characters for Telegram
func escapeHTML(s string) string {
	replacer := map[rune]string{
		'&':  "&amp;",
		'<':  "&lt;",
		'>':  "&gt;",
		'"':  "&quot;",
		'\'': "&#39;",
	}

	result := ""
	for _, char := range s {
		if replacement, ok := replacer[char]; ok {
			result += replacement
		} else {
			result += string(char)
		}
	}
	return result
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getOrDefault returns the value if not empty, otherwise returns the default
func getOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
