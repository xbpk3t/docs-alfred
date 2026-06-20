package transcript

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const (
	xiaoyuzhouAPIBase      = "https://api.xiaoyuzhoufm.com"
	xiaoyuzhouUserAgent    = "Xiaoyuzhou/2.98.0 (build:2908; iOS 26.2.1)"
	xiaoyuzhouDefaultDevID = "81ADBFD6-6921-482B-9AB9-A29E7CC7BB55"
	xiaoyuzhouTimeout      = 20 * time.Second
)

var episodeIDRe = regexp.MustCompile(`xiaoyuzhoufm\.com/episode/([a-f0-9]+)`)

// XiaoyuzhouCredentialFile returns the default credential file path.
func XiaoyuzhouCredentialFile() string {
	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".opencli", "xiaoyuzhou.json")
}

// xiaoyuzhouCredentials mirrors the credential file format.
type xiaoyuzhouCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
	ExpiresAt    int64  `json:"expires_at"`
}

// XiaoyuzhouProvider fetches transcripts from the Xiaoyuzhou API.
type XiaoyuzhouProvider struct {
	credentialPath string
	mu             sync.Mutex
}

// NewXiaoyuzhouProvider creates a new XiaoyuzhouProvider.
// If credentialPath is empty, uses ~/.opencli/xiaoyuzhou.json.
func NewXiaoyuzhouProvider(credentialPath string) *XiaoyuzhouProvider {
	if credentialPath == "" {
		credentialPath = XiaoyuzhouCredentialFile()
	}

	return &XiaoyuzhouProvider{credentialPath: credentialPath}
}

func (p *XiaoyuzhouProvider) Name() string {
	return "xiaoyuzhou"
}

// ValidateCredentials checks if the xiaoyuzhou credentials are usable
// by attempting a token refresh. Returns nil if valid.
func (p *XiaoyuzhouProvider) ValidateCredentials(ctx context.Context) error {
	_, err := p.refreshAndGetToken(ctx)

	return err
}

func (p *XiaoyuzhouProvider) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error) {
	eid := extractEpisodeID(ep.URL, ep.GUID)
	if eid == "" {
		return nil, errors.New("cannot extract xiaoyuzhou episode ID from URL/GUID")
	}

	token, err := p.refreshAndGetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("xiaoyuzhou auth: %w", err)
	}

	mediaID, err := p.apiGetEpisode(ctx, eid, token)
	if err != nil {
		return nil, fmt.Errorf("xiaoyuzhou get episode: %w", err)
	}

	transcriptURL, err := p.apiGetTranscriptURL(ctx, eid, mediaID, token)
	if err != nil {
		return nil, fmt.Errorf("xiaoyuzhou get transcript url: %w", err)
	}

	body, err := p.fetchTranscriptBody(ctx, transcriptURL)
	if err != nil {
		return nil, fmt.Errorf("xiaoyuzhou fetch transcript: %w", err)
	}

	text, _ := extractTranscriptText(body)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("xiaoyuzhou transcript is empty")
	}

	return &TranscriptResult{
		Content:     strings.TrimSpace(text),
		ContentType: plaintextContentType,
		Source:      "xiaoyuzhou",
	}, nil
}

// --- Token management ---

func (p *XiaoyuzhouProvider) refreshAndGetToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	creds, err := loadCredentials(p.credentialPath)
	if err != nil {
		return "", err
	}

	newCreds, err := refreshToken(ctx, creds)
	if err != nil {
		return "", err
	}

	if err := saveCredentials(p.credentialPath, newCreds); err != nil {
		return "", fmt.Errorf("save credentials: %w", err)
	}

	return newCreds.AccessToken, nil
}

func loadCredentials(path string) (*xiaoyuzhouCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credential file %s: %w", path, err)
	}
	var creds xiaoyuzhouCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credential file: %w", err)
	}
	if creds.RefreshToken == "" {
		return nil, errors.New("credential file missing refresh_token")
	}
	if creds.DeviceID == "" {
		creds.DeviceID = xiaoyuzhouDefaultDevID
	}

	return &creds, nil
}

func saveCredentials(path string, creds *xiaoyuzhouCredentials) error {
	data, err := json.MarshalIndent(creds, "", "  ") //nolint:gosec // G117: intentional credential file write
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func refreshToken(ctx context.Context, creds *xiaoyuzhouCredentials) (*xiaoyuzhouCredentials, error) {
	rc := httputil.NewRestyClient(xiaoyuzhouTimeout, 0)
	req := rc.R().SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded; charset=utf-8").
		SetHeader("Host", "api.xiaoyuzhoufm.com").
		SetHeader("User-Agent", xiaoyuzhouUserAgent).
		SetHeader("Market", "AppStore").
		SetHeader("App-BuildNo", "2908").
		SetHeader("OS", "ios").
		SetHeader("Manufacturer", "Apple").
		SetHeader("BundleID", "app.podcast.cosmos").
		SetHeader("Accept", "*/*").
		SetHeader("App-Version", "2.98.0").
		SetHeader("OS-Version", "26.2.1").
		SetHeader("x-jike-device-id", creds.DeviceID).
		SetHeader("x-jike-refresh-token", creds.RefreshToken).
		SetHeader("Local-Time", time.Now().UTC().Format(time.RFC3339))

	resp, err := req.Post(xiaoyuzhouAPIBase + "/app_auth_tokens.refresh")
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("refresh failed: HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result struct {
		AccessToken string `json:"x-jike-access-token"`
		RefreshTkn  string `json:"x-jike-refresh-token"`
		Success     bool   `json:"success"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}
	if !result.Success {
		slog.Warn("Xiaoyuzhou refresh API returned success=false",
			"body", string(resp.Body()),
		)

		return nil, errors.New("refresh API returned success=false")
	}
	if result.AccessToken == "" || result.RefreshTkn == "" {
		return nil, errors.New("refresh API returned empty tokens")
	}

	return &xiaoyuzhouCredentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshTkn,
		ExpiresAt:    time.Now().UnixMilli() + 20*60*1000,
		DeviceID:     creds.DeviceID,
	}, nil
}

// --- API calls ---

func (p *XiaoyuzhouProvider) apiGetEpisode(ctx context.Context, eid, token string) (string, error) {
	rc := httputil.NewRestyClient(xiaoyuzhouTimeout, 0)
	req := rc.R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Host", "api.xiaoyuzhoufm.com").
		SetHeader("User-Agent", xiaoyuzhouUserAgent).
		SetHeader("Accept", "*/*").
		SetHeader("x-jike-access-token", token).
		SetHeader("x-jike-device-id", xiaoyuzhouDefaultDevID).
		SetQueryParam("eid", eid)

	resp, err := req.Get(xiaoyuzhouAPIBase + "/v1/episode/get")
	if err != nil {
		return "", fmt.Errorf("get episode: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("get episode: HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result struct {
		Data struct {
			Transcript struct {
				MediaID string `json:"mediaId"`
			} `json:"transcript"`
			Media struct {
				ID string `json:"id"`
			} `json:"media"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("parse episode response: %w", err)
	}
	if !result.Success {
		slog.Warn("Xiaoyuzhou episode API returned success=false",
			"eid", eid,
			"status", resp.StatusCode(),
			"body", string(resp.Body()),
		)

		return "", errors.New("episode API returned success=false")
	}

	mediaID := result.Data.Transcript.MediaID
	if mediaID == "" {
		mediaID = result.Data.Media.ID
	}
	if mediaID == "" {
		return "", errors.New("mediaId not found in episode response")
	}

	return mediaID, nil
}

func (p *XiaoyuzhouProvider) apiGetTranscriptURL(ctx context.Context, eid, mediaID, token string) (string, error) {
	rc := httputil.NewRestyClient(xiaoyuzhouTimeout, 0)
	req := rc.R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Host", "api.xiaoyuzhoufm.com").
		SetHeader("User-Agent", xiaoyuzhouUserAgent).
		SetHeader("Accept", "*/*").
		SetHeader("x-jike-access-token", token).
		SetHeader("x-jike-device-id", xiaoyuzhouDefaultDevID).
		SetBody(map[string]string{"eid": eid, "mediaId": mediaID})

	resp, err := req.Post(xiaoyuzhouAPIBase + "/v1/episode-transcript/get")
	if err != nil {
		return "", fmt.Errorf("get transcript url: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("get transcript url: HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result struct {
		Data struct {
			TranscriptURL string `json:"transcriptUrl"`
			URL           string `json:"url"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("parse transcript url response: %w", err)
	}
	if !result.Success {
		slog.Warn("Xiaoyuzhou transcript URL API returned success=false",
			"eid", eid,
			"status", resp.StatusCode(),
			"body", string(resp.Body()),
		)

		return "", errors.New("transcript url API returned success=false")
	}

	url := result.Data.TranscriptURL
	if url == "" {
		url = result.Data.URL
	}
	if url == "" {
		return "", errors.New("transcriptUrl not found in response")
	}

	return url, nil
}

func (p *XiaoyuzhouProvider) fetchTranscriptBody(ctx context.Context, url string) ([]byte, error) {
	return httputil.GetBytes(ctx, url, httputil.RequestOptions{
		Headers: map[string]string{
			"User-Agent": xiaoyuzhouUserAgent,
			"Accept":     "*/*",
			"Market":     "AppStore",
		},
		Timeout: xiaoyuzhouTimeout,
	})
}

// --- Helpers ---

// extractEpisodeID extracts the episode ID from a xiaoyuzhou URL or GUID.
func extractEpisodeID(url, guid string) string {
	for _, s := range []string{guid, url} {
		if m := episodeIDRe.FindStringSubmatch(s); len(m) > 1 {
			return m[1]
		}
	}

	return ""
}

// xiaoyuzhouTranscriptSegment represents one segment in the transcript JSON.
type xiaoyuzhouTranscriptSegment struct {
	Text    string `json:"text"`
	StartMs int64  `json:"startMs"`
}

// extractTranscriptText extracts plain text from xiaoyuzhou transcript JSON.
func extractTranscriptText(body []byte) (string, int) {
	segments := parseTranscriptSegments(body)

	var lines []string
	for _, seg := range segments {
		if t := strings.TrimSpace(seg.Text); t != "" {
			lines = append(lines, t)
		}
	}

	return strings.Join(lines, "\n"), len(lines)
}

func parseTranscriptSegments(body []byte) []xiaoyuzhouTranscriptSegment {
	var segments []xiaoyuzhouTranscriptSegment
	if err := json.Unmarshal(body, &segments); err == nil {
		return segments
	}

	// Try wrapping in object: {"segments": [...]} or {"data": [...]}
	var wrapper struct {
		Segments []xiaoyuzhouTranscriptSegment `json:"segments"`
		Data     []xiaoyuzhouTranscriptSegment `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil
	}
	if len(wrapper.Segments) > 0 {
		return wrapper.Segments
	}

	return wrapper.Data
}
