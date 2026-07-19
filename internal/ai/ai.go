// Package ai generates the "AI Alert & Action Plan" via OpenRouter
// (https://openrouter.ai). It is grounded strictly on the current dashboard
// payload and returns the same domain.Alert shape the rule-based engine uses,
// so the frontend renders it identically. When no API key is configured (or a
// call fails) the caller falls back to the rule-based alerts.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"greenpark/sales/internal/domain"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"

const systemPrompt = `Kamu adalah analis sales senior Greenpark Group untuk dashboard "Control Tower".
Tugasmu: membaca DATA SALES (JSON) dan menghasilkan "AI Alert & Action Plan" — daftar
prioritas eksekusi yang ringkas dan actionable, HANYA berdasarkan data tersebut.

Aturan keluaran:
- Balas HANYA dengan JSON array valid, tanpa teks lain, tanpa markdown/code fence.
- Maksimal 6 item, diurutkan dari paling kritis ke peluang.
- Setiap item objek dengan field:
  "sev"      : "merah" (kritis) | "kuning" (risiko) | "hijau" (peluang)
  "title"    : ringkas + angka kunci, mis "Funnel Leads → CV rendah (5.5%, target ≥20%)"
  "detail"   : 1 kalimat menjelaskan kondisi dari data (sebut angka nyata).
  "pic"      : penanggung jawab, mis "Sales / Kadep", "Marketing / SPV", "KPR / Finance".
  "deadline" : "Hari ini" | "H+3" | "Minggu ini" | "Mingguan".
  "action"   : 1 kalimat rekomendasi tindakan konkret.
- Bahasa Indonesia, profesional, format uang sebagai Rupiah. Jangan mengarang angka di luar data.`

// Client calls OpenRouter chat completions.
type Client struct {
	key   string
	model string
	site  string
	http  *http.Client
}

// New builds a client. It is always non-nil; Configured() reports usability.
func New(key, model, site string) *Client {
	if model == "" {
		model = "openai/gpt-oss-120b:free"
	}
	return &Client{key: key, model: model, site: site, http: &http.Client{Timeout: 110 * time.Second}}
}

// Configured reports whether an API key is present.
func (c *Client) Configured() bool { return c != nil && strings.TrimSpace(c.key) != "" }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Alerts asks the model to analyse the dashboard and return prioritised alerts.
func (c *Client) Alerts(ctx context.Context, d *domain.Dashboard) ([]domain.Alert, error) {
	grounding, err := buildGrounding(d)
	if err != nil {
		return nil, err
	}
	content, err := c.complete(ctx, systemPrompt,
		"DATA SALES (JSON, sumber kebenaran):\n"+grounding+
			"\n\nHasilkan AI Alert & Action Plan sebagai JSON array sekarang.",
		0.3, 900)
	if err != nil {
		return nil, err
	}
	return parseAlerts(content)
}

// complete runs one chat completion and returns the assistant message content.
// Shared by Alerts and Screen so the OpenRouter wire plumbing lives in one place.
func (c *Client) complete(ctx context.Context, system, user string, temperature float64, maxTokens int) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("OpenRouter belum dikonfigurasi (set OPENROUTER_API_KEY)")
	}

	reqBody, _ := json.Marshal(chatRequest{
		Model:       c.model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.key)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", c.site)
	httpReq.Header.Set("X-Title", "Greenpark Sales Control Tower")

	res, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var parsed chatResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("OpenRouter: gagal baca respons: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		msg := "status " + res.Status
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return "", fmt.Errorf("OpenRouter %d: %s", res.StatusCode, msg)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter: respons kosong")
	}
	return parsed.Choices[0].Message.Content, nil
}

// buildGrounding marshals the decision-relevant slice of the dashboard (without
// the heavy per-project breakdown) as compact JSON for the prompt.
func buildGrounding(d *domain.Dashboard) (string, error) {
	payload := map[string]any{
		"period":   d.Period,
		"exec":     d.Exec,
		"funnel":   d.Funnel,
		"projects": d.Projects,
		"channels": d.Channels,
		"sales":    d.Sales,
		"reasons":  d.Reasons,
		"summary":  d.Summary,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseAlerts extracts the JSON array from the model output (tolerating stray
// prose or ```json fences) and maps it onto domain.Alert.
func parseAlerts(content string) ([]domain.Alert, error) {
	s := strings.TrimSpace(content)
	// Strip a leading/trailing markdown fence if present.
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			s = s[i : j+1]
		}
	}
	var raw []struct {
		Sev      string `json:"sev"`
		Title    string `json:"title"`
		Detail   string `json:"detail"`
		PIC      string `json:"pic"`
		Deadline string `json:"deadline"`
		Action   string `json:"action"`
	}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("AI: format keluaran tidak terbaca: %w", err)
	}
	out := make([]domain.Alert, 0, len(raw))
	for _, r := range raw {
		sev := strings.ToLower(strings.TrimSpace(r.Sev))
		if sev != "merah" && sev != "kuning" && sev != "hijau" {
			sev = "kuning"
		}
		if strings.TrimSpace(r.Title) == "" {
			continue
		}
		out = append(out, domain.Alert{
			Sev: sev, Title: r.Title, Detail: r.Detail,
			PIC: r.PIC, Deadline: r.Deadline, Action: r.Action,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("AI: tidak ada alert yang dihasilkan")
	}
	return out, nil
}
