package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiURL = "https://api.anthropic.com/v1/messages"
	model  = "claude-sonnet-4-20250514"
)

// GetAPIKey returns the Anthropic API key from the environment or config value.
func GetAPIKey(configKey string) string {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return configKey
}

// SummarizeSession generates a plain-English summary for a coding session diff.
func SummarizeSession(diff, projectName, apiKey string) (string, error) {
	if strings.TrimSpace(diff) == "" {
		return "No code changes detected in this session.", nil
	}

	prompt := fmt.Sprintf(`You are summarizing a developer's coding session for their personal log.

Here is the git diff of changes made during this session:
<diff>
%s
</diff>

Write a 2-4 sentence plain English summary of:
1. What was built or changed
2. What the code appears to do
3. What files or tech stack was involved

Be specific and concrete. Don't use filler phrases like "the developer worked on..."
Just describe what was built as if writing a commit message narrative.
Keep it under 60 words.`, diff)

	return callClaude(prompt, apiKey)
}

// SummarizeWeek generates a rolled-up bullet-point summary of a week's session summaries.
func SummarizeWeek(summaries []string, apiKey string) (string, error) {
	if len(summaries) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for i, s := range summaries {
		fmt.Fprintf(&sb, "Session %d: %s\n", i+1, s)
	}

	prompt := fmt.Sprintf(`You are summarizing a developer's week of coding sessions for their personal log.

Here are the individual session summaries from this week:
%s

Write a concise bullet-point list (3-6 bullets) of what was shipped this week.
Each bullet should describe a distinct feature, fix, or area of work.
Be specific and concrete. Start each bullet with a past-tense verb (e.g. "Built", "Fixed", "Added").
Do not include a header — just the bullets.`, sb.String())

	return callClaude(prompt, apiKey)
}

// ──────────────────────────────────────────────────────────────────
// Internal HTTP client
// ──────────────────────────────────────────────────────────────────

type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func callClaude(prompt, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no Anthropic API key set — run: breaklog config set api-key <key>")
	}

	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: 512,
		Messages:  []message{{Role: "user", Content: prompt}},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var cr claudeResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", cr.Error.Message)
	}
	if len(cr.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return strings.TrimSpace(cr.Content[0].Text), nil
}
