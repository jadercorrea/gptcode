package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type SentryClient struct {
	dsn        string
	org        string
	project    string
	token      string
	httpClient *http.Client
}

func NewSentryClient() *SentryClient {
	return &SentryClient{
		dsn:        os.Getenv("SENTRY_DSN"),
		org:        os.Getenv("SENTRY_ORG"),
		project:    os.Getenv("SENTRY_PROJECT"),
		token:      os.Getenv("SENTRY_AUTH_TOKEN"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SentryClient) IsConfigured() bool {
	return s.dsn != "" && s.token != ""
}

type SentryIssue struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ShortID   string `json:"shortId"`
	Level     string `json:"level"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Project   string `json:"project"`
	Status    string `json:"status"`
	Culprit   string `json:"culprit"`
	Platform  string `json:"platform"`
	Count     string `json:"count"`
	UserCount string `json:"userCount"`
	IsUnread  bool   `json:"isUnread"`
}

func (s *SentryClient) ListIssues(ctx context.Context, query string) ([]SentryIssue, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("sentry not configured")
	}

	url := fmt.Sprintf("https://sentry.io/api/0/organizations/%s/issues/", s.org)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+s.token)
	q := req.URL.Query()
	q.Add("query", query)
	q.Add("limit", "20")
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var issues []SentryIssue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, err
	}

	return issues, nil
}

func (s *SentryClient) GetIssue(ctx context.Context, issueID string) (*SentryIssue, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("sentry not configured")
	}

	url := fmt.Sprintf("https://sentry.io/api/0/organizations/%s/issues/%s/", s.org, issueID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var issue SentryIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

func (s *SentryClient) GetLatestErrors(ctx context.Context, hours int) ([]SentryIssue, error) {
	query := fmt.Sprintf("firstSeen:>-%dh", hours)
	return s.ListIssues(ctx, query)
}

type SentryEvent struct {
	ID          string                 `json:"id"`
	IssueID     string                 `json:"issue_id"`
	Timestamp   string                 `json:"timestamp"`
	Platform    string                 `json:"platform"`
	Environment string                 `json:"environment"`
	Message     string                 `json:"message"`
	Logger      string                 `json:"logger"`
	Level       string                 `json:"level"`
	Contexts    map[string]interface{} `json:"contexts"`
}

func (s *SentryClient) GetEvents(ctx context.Context, issueID string) ([]SentryEvent, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("sentry not configured")
	}

	url := fmt.Sprintf("https://sentry.io/api/0/organizations/%s/issues/%s/events/", s.org, issueID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var events []SentryEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, err
	}

	return events, nil
}

type TelegramClient struct {
	botToken string
	chatID   string
}

func NewTelegramClient() *TelegramClient {
	return &TelegramClient{
		botToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		chatID:   os.Getenv("TELEGRAM_CHAT_ID"),
	}
}

func (t *TelegramClient) IsConfigured() bool {
	return t.botToken != "" && t.chatID != ""
}

type TelegramMessage struct {
	ChatID            string      `json:"chat_id"`
	Text              string      `json:"text"`
	ParseMode         string      `json:"parse_mode,omitempty"`
	DisableWebPreview bool        `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup       interface{} `json:"reply_markup,omitempty"`
}

func (t *TelegramClient) Send(ctx context.Context, message string) error {
	if !t.IsConfigured() {
		return fmt.Errorf("telegram not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	msg := TelegramMessage{
		ChatID:    t.chatID,
		Text:      message,
		ParseMode: "Markdown",
	}

	body, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (t *TelegramClient) SendError(ctx context.Context, title, details string) error {
	message := fmt.Sprintf("🔴 *Error Detected*\n\n*%s*\n\n```\n%s\n```", title, truncate(details, 500))
	return t.Send(ctx, message)
}

func (t *TelegramClient) SendWarning(ctx context.Context, title, details string) error {
	message := fmt.Sprintf("⚠️ *Warning*\n\n*%s*\n\n```\n%s\n```", title, truncate(details, 500))
	return t.Send(ctx, message)
}

func (t *TelegramClient) SendInfo(ctx context.Context, title, details string) error {
	message := fmt.Sprintf("ℹ️ *Info*\n\n*%s*\n\n%s", title, truncate(details, 500))
	return t.Send(ctx, message)
}

func (t *TelegramClient) SendSuccess(ctx context.Context, title, details string) error {
	message := fmt.Sprintf("✅ *Success*\n\n*%s*\n\n%s", title, truncate(details, 500))
	return t.Send(ctx, message)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type BugNotifier struct {
	sentry   *SentryClient
	telegram *TelegramClient
}

func NewBugNotifier() *BugNotifier {
	return &BugNotifier{
		sentry:   NewSentryClient(),
		telegram: NewTelegramClient(),
	}
}

func (bn *BugNotifier) CheckAndNotify(ctx context.Context, hours int) error {
	issues, err := bn.sentry.GetLatestErrors(ctx, hours)
	if err != nil {
		return err
	}

	var critical []SentryIssue
	for _, issue := range issues {
		if issue.Level == "error" && issue.IsUnread {
			critical = append(critical, issue)
		}
	}

	if len(critical) > 0 {
		title := fmt.Sprintf("%d new errors detected", len(critical))
		var details strings.Builder
		for _, issue := range critical {
			fmt.Fprintf(&details, "- [%s] %s\n", issue.ShortID, issue.Title)
		}
		bn.telegram.SendError(ctx, title, details.String())
	}

	return nil
}

func (bn *BugNotifier) Watch(ctx context.Context, interval time.Duration, callback func([]SentryIssue)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			issues, err := bn.sentry.GetLatestErrors(ctx, 1)
			if err != nil {
				continue
			}
			callback(issues)
		}
	}
}

type GitHubIssueCreator struct {
	owner string
	repo  string
	token string
}

func NewGitHubIssueCreator(owner, repo string) *GitHubIssueCreator {
	return &GitHubIssueCreator{
		owner: owner,
		repo:  repo,
		token: os.Getenv("GITHUB_TOKEN"),
	}
}

type GitHubIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
}

func (g *GitHubIssueCreator) Create(ctx context.Context, title, body string, labels []string) (*GitHubIssue, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", g.owner, g.repo)

	issueData := map[string]interface{}{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		issueData["labels"] = labels
	}

	bodyJSON, _ := json.Marshal(issueData)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var issue GitHubIssue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

func (g *GitHubIssueCreator) CreateFromSentryIssue(ctx context.Context, sentryIssue *SentryIssue) (*GitHubIssue, error) {
	title := fmt.Sprintf("[Sentry] %s", sentryIssue.Title)
	body := fmt.Sprintf(`## Sentry Issue

- **ID:** %s
- **Project:** %s
- **First Seen:** %s
- **Last Seen:** %s
- **Count:** %s
- **User Count:** %s
- **Status:** %s

## Culprit

%s

---
Auto-created from Sentry`, sentryIssue.ID, sentryIssue.Project, sentryIssue.FirstSeen, sentryIssue.LastSeen, sentryIssue.Count, sentryIssue.UserCount, sentryIssue.Status, sentryIssue.Culprit)

	return g.Create(ctx, title, body, []string{"bug", "sentry", "auto-created"})
}
