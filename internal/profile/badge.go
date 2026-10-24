package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// BadgeText is the four-character toolbar badge for a profile id.
func BadgeText(id string) string {
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "AB"
	}
	runes := []rune(s)
	if len(runes) > 4 {
		runes = runes[len(runes)-4:]
	}
	return strings.ToUpper(string(runes))
}

// WriteBadge writes a tiny MV3 extension that shows the profile id on the toolbar.
func WriteBadge(dir, profileID, email string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	title := profileID
	if email != "" {
		title = profileID + " · " + email
	}
	manifest := map[string]any{
		"manifest_version": 3,
		"name":             "Agent Browser — " + profileID,
		"version":          "1.0.0",
		"description":      "Shows which Agent Browser profile this window belongs to.",
		"action": map[string]any{
			"default_title": title,
		},
		"background": map[string]any{
			"service_worker": "sw.js",
		},
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o644); err != nil {
		return err
	}
	idJSON, _ := json.Marshal(profileID)
	emailJSON, _ := json.Marshal(email)
	badgeJSON, _ := json.Marshal(BadgeText(profileID))
	js := strings.ReplaceAll(badgeWorker, "__ID__", string(idJSON))
	js = strings.ReplaceAll(js, "__EMAIL__", string(emailJSON))
	js = strings.ReplaceAll(js, "__BADGE__", string(badgeJSON))
	return os.WriteFile(filepath.Join(dir, "sw.js"), []byte(js), 0o644)
}

const badgeWorker = `const PROFILE_ID = __ID__;
const PROFILE_EMAIL = __EMAIL__;
const BADGE = __BADGE__;

function show() {
  const title = PROFILE_EMAIL ? PROFILE_ID + " · " + PROFILE_EMAIL : PROFILE_ID;
  chrome.action.setBadgeText({ text: BADGE });
  chrome.action.setBadgeBackgroundColor({ color: "#0B6E4F" });
  chrome.action.setTitle({ title: title });
}

chrome.runtime.onInstalled.addListener(show);
chrome.runtime.onStartup.addListener(show);
show();
`
