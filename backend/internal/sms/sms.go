package sms

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type SMS struct {
	accountSID string
	authToken  string
	from       string
}

func New(accountSID, authToken, from string) *SMS {
	return &SMS{accountSID: accountSID, authToken: authToken, from: from}
}

// Configured returns true when Twilio credentials are set.
func (s *SMS) Configured() bool {
	return s.accountSID != "" && s.authToken != "" && s.from != ""
}

// Send dispatches a text message via Twilio. Returns silently if not configured.
func (s *SMS) Send(to, body string) error {
	if !s.Configured() || to == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)

	form := url.Values{}
	form.Set("From", s.from)
	form.Set("To", to)
	form.Set("Body", body)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	creds := base64.StdEncoding.EncodeToString([]byte(s.accountSID + ":" + s.authToken))
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("sms: twilio error", "status", resp.StatusCode, "to", to)
	}
	return nil
}
