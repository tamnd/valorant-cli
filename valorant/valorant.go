// Package valorant is the library behind the valorant command line:
// the HTTP client, request shaping, and the typed data models for the
// valorant-api.com static game data API.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package valorant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to valorant-api.com.
const DefaultUserAgent = "valorant-cli/0.1 (tamnd87@gmail.com)"

// Host is the API hostname this client talks to.
const Host = "valorant-api.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://valorant-api.com/v1"

// DefaultLanguage is the language tag sent to the API.
const DefaultLanguage = "en-US"

// Client talks to valorant-api.com over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	Language  string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		UserAgent: DefaultUserAgent,
		Language:  DefaultLanguage,
		Rate:      200 * time.Millisecond,
		Retries:   3,
	}
}

// Get fetches rawURL and returns the response body. It paces and retries
// according to the client's settings.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// wire envelope for all responses.
type wireResp[T any] struct {
	Status int `json:"status"`
	Data   T   `json:"data"`
}

// --- wire types ---

type wireAgent struct {
	UUID                string `json:"uuid"`
	DisplayName         string `json:"displayName"`
	Description         string `json:"description"`
	IsPlayableCharacter bool   `json:"isPlayableCharacter"`
	Role                *struct {
		DisplayName string `json:"displayName"`
	} `json:"role"`
	Abilities []struct {
		Slot        string `json:"slot"`
		DisplayName string `json:"displayName"`
	} `json:"abilities"`
}

type wireWeapon struct {
	UUID        string `json:"uuid"`
	DisplayName string `json:"displayName"`
	ShopData    *struct {
		Cost     int    `json:"cost"`
		Category string `json:"category"`
	} `json:"shopData"`
	WeaponStats *struct {
		FireRate          float64 `json:"fireRate"`
		MagazineSize      int     `json:"magazineSize"`
		ReloadTimeSeconds float64 `json:"reloadTimeSeconds"`
	} `json:"weaponStats"`
}

type wireMap struct {
	UUID                string `json:"uuid"`
	DisplayName         string `json:"displayName"`
	NarrativeDescription string `json:"narrativeDescription"`
	TacticalDescription string `json:"tacticalDescription"`
	Coordinates         string `json:"coordinates"`
}

type wireTierSet struct {
	Tiers []struct {
		Tier         int    `json:"tier"`
		TierName     string `json:"tierName"`
		DivisionName string `json:"divisionName"`
	} `json:"tiers"`
}

// buildURL constructs an API URL with language set.
func (c *Client) buildURL(path string, extra ...string) string {
	u, _ := url.Parse(BaseURL + path)
	q := u.Query()
	lang := c.Language
	if lang == "" {
		lang = DefaultLanguage
	}
	q.Set("language", lang)
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Agents fetches playable agents. If all is true, include non-playable ones too.
func (c *Client) Agents(ctx context.Context, all bool) ([]*Agent, error) {
	extra := []string{}
	if !all {
		extra = []string{"isPlayableCharacter", "true"}
	}
	rawURL := c.buildURL("/agents", extra...)
	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp wireResp[[]wireAgent]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode agents: %w", err)
	}
	out := make([]*Agent, 0, len(resp.Data))
	for _, a := range resp.Data {
		role := ""
		if a.Role != nil {
			role = a.Role.DisplayName
		}
		abilities := make([]string, 0, len(a.Abilities))
		for _, ab := range a.Abilities {
			if ab.DisplayName != "" {
				abilities = append(abilities, ab.DisplayName)
			}
		}
		out = append(out, &Agent{
			UUID:        a.UUID,
			Name:        a.DisplayName,
			Role:        role,
			Description: a.Description,
			Abilities:   strings.Join(abilities, ", "),
		})
	}
	return out, nil
}

// Weapons fetches all weapons. Pass category="" to get all.
func (c *Client) Weapons(ctx context.Context, category string) ([]*Weapon, error) {
	rawURL := c.buildURL("/weapons")
	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp wireResp[[]wireWeapon]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode weapons: %w", err)
	}
	out := make([]*Weapon, 0, len(resp.Data))
	for _, w := range resp.Data {
		cat := ""
		cost := 0
		if w.ShopData != nil {
			cat = w.ShopData.Category
			cost = w.ShopData.Cost
		}
		var fireRate float64
		var magSize int
		var reloadTime float64
		if w.WeaponStats != nil {
			fireRate = w.WeaponStats.FireRate
			magSize = w.WeaponStats.MagazineSize
			reloadTime = w.WeaponStats.ReloadTimeSeconds
		}
		if category != "" && !strings.EqualFold(cat, category) {
			continue
		}
		out = append(out, &Weapon{
			UUID:       w.UUID,
			Name:       w.DisplayName,
			Category:   cat,
			Cost:       cost,
			FireRate:   fireRate,
			MagSize:    magSize,
			ReloadTime: reloadTime,
		})
	}
	return out, nil
}

// Maps fetches all maps.
func (c *Client) Maps(ctx context.Context) ([]*Map, error) {
	rawURL := c.buildURL("/maps")
	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp wireResp[[]wireMap]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode maps: %w", err)
	}
	out := make([]*Map, 0, len(resp.Data))
	for _, m := range resp.Data {
		out = append(out, &Map{
			UUID:                m.UUID,
			Name:                m.DisplayName,
			TacticalDescription: m.TacticalDescription,
			Coordinates:         m.Coordinates,
		})
	}
	return out, nil
}

// Ranks fetches competitive rank tiers from the most recent episode.
func (c *Client) Ranks(ctx context.Context) ([]*Rank, error) {
	rawURL := c.buildURL("/competitivetiers")
	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp wireResp[[]wireTierSet]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode competitivetiers: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no competitive tier sets found")
	}
	// Use the last (most recent) episode.
	last := resp.Data[len(resp.Data)-1]
	out := make([]*Rank, 0, len(last.Tiers))
	for _, t := range last.Tiers {
		// Skip placeholder/unused tiers and unranked (tier 0).
		name := strings.ToLower(t.TierName)
		if t.TierName == "" || t.Tier == 0 || strings.HasPrefix(name, "unused") {
			continue
		}
		out = append(out, &Rank{
			Tier:     t.Tier,
			Name:     t.TierName,
			Division: t.DivisionName,
		})
	}
	return out, nil
}
