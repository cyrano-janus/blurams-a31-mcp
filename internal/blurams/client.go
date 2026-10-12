package blurams

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultProductKey    = "45c8bd2a-e70"
	defaultProductSecret = "qqg5C36lYBlDAQvp3XL6"
)

type Camera struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Model        string `json:"model,omitempty"`
	Online       bool   `json:"online"`
	SupportsPan  bool   `json:"supports_pan"`
	SupportsTilt bool   `json:"supports_tilt"`
	ProductKey   string `json:"-"`
}

type MoveResult struct {
	CameraID  string `json:"camera_id"`
	Direction string `json:"direction"`
	Steps     int    `json:"steps"`
	Accepted  bool   `json:"accepted"`
}

type Client struct {
	email         string
	password      string
	apiHost       string
	relayURL      string
	productKey    string
	productSecret string
	httpClient    *http.Client
	now           func() time.Time

	mu    sync.Mutex
	token string
}

type Options struct {
	APIHost       string
	RelayURL      string
	ProductKey    string
	ProductSecret string
	HTTPClient    *http.Client
	Now           func() time.Time
}

func NewClient(email, password string, opts Options) *Client {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	productKey := opts.ProductKey
	if productKey == "" {
		productKey = defaultProductKey
	}
	productSecret := opts.ProductSecret
	if productSecret == "" {
		productSecret = defaultProductSecret
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		email: email, password: password, apiHost: strings.TrimRight(opts.APIHost, "/"),
		relayURL: opts.RelayURL, productKey: productKey, productSecret: productSecret,
		httpClient: client, now: now,
	}
}

type apiEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) post(ctx context.Context, path string, payload any, token string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode vendor request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiHost+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create vendor request: %w", err)
	}
	for key, value := range c.headers(path, string(body), token) {
		req.Header.Set(key, value)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vendor request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vendor returned HTTP %d", resp.StatusCode)
	}
	var envelope apiEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode vendor response: %w", err)
	}
	if envelope.Code != 200 {
		if envelope.Msg == "" {
			envelope.Msg = "vendor API rejected the request"
		}
		return errors.New(envelope.Msg)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode vendor data: %w", err)
	}
	return nil
}

func (c *Client) headers(path, body, token string) map[string]string {
	timestamp := strconv.FormatInt(c.now().UnixMilli(), 10)
	nonce := randomHex(16)
	headers := map[string]string{
		"User-Agent":       "okhttp/4.9.2",
		"Request-Version":  "1",
		"Accept-Time":      timestamp,
		"Accept-AccessKey": c.productKey,
		"Accept-Nonce":     nonce,
		"Accept-Sign":      requestSignature(c.productKey, timestamp, path, c.productSecret, body, nonce),
		"Content-Type":     "application/json; charset=utf-8",
	}
	if token != "" {
		headers["userToken"] = token
	}
	return headers
}

func randomHex(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		panic("cryptographic random source unavailable")
	}
	return hex.EncodeToString(value)
}

func (c *Client) login(ctx context.Context) (string, error) {
	var data struct {
		UserToken string `json:"userToken"`
		Token     string `json:"token"`
	}
	payload := map[string]any{"data": map[string]any{
		"account": c.email, "accountType": "email", "deviceId": randomHex(16),
		"password": passwordDigest(c.password), "passwordTypeEnum": "enc1", "loc": "de_DE",
	}}
	if err := c.post(ctx, "/oauth/user/login", payload, "", &data); err != nil {
		return "", err
	}
	token := data.UserToken
	if token == "" {
		token = data.Token
	}
	if token == "" {
		return "", errors.New("login response contained no usable token")
	}
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	return token, nil
}

func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		return token, nil
	}
	return c.login(ctx)
}

func (c *Client) ListCameras(ctx context.Context) ([]Camera, error) {
	token, err := c.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	var data struct {
		DeviceList []struct {
			DeviceID     string `json:"deviceId"`
			DeviceName   string `json:"deviceName"`
			DeviceTitle  string `json:"deviceTitle"`
			CameraModel  string `json:"cameraModelId"`
			OnlineStatus string `json:"onlineStatus"`
			Comment      any    `json:"comment"`
		} `json:"deviceList"`
	}
	payload := map[string]any{"data": map[string]any{
		"pageSize": 50, "settingPaths": []string{}, "supportPaths": []string{},
		"attributeIdList": []string{},
	}}
	if err := c.post(ctx, "/sclient/compatible/device/list", payload, token, &data); err != nil {
		return nil, err
	}
	cameras := make([]Camera, 0, len(data.DeviceList))
	for _, raw := range data.DeviceList {
		if raw.DeviceID == "" {
			continue
		}
		name := raw.DeviceName
		if name == "" {
			name = raw.DeviceTitle
		}
		if name == "" {
			name = raw.DeviceID
		}
		pan, tilt := capabilities(fmt.Sprint(raw.Comment))
		cameras = append(cameras, Camera{
			ID: raw.DeviceID, Name: name, Model: raw.CameraModel,
			Online: raw.OnlineStatus == "available", ProductKey: c.productKey,
			SupportsPan: pan, SupportsTilt: tilt,
		})
	}
	return cameras, nil
}

func (c *Client) Move(ctx context.Context, camera Camera, direction string, steps int) (MoveResult, error) {
	normalized, value, err := normalizeMove(direction, steps)
	if err != nil {
		return MoveResult{}, err
	}
	token, err := c.ensureToken(ctx)
	if err != nil {
		return MoveResult{}, err
	}
	for range steps {
		if err := c.moveOnce(ctx, token, camera.ID, camera.ProductKey, value); err != nil {
			return MoveResult{}, err
		}
	}
	return MoveResult{CameraID: camera.ID, Direction: normalized, Steps: steps, Accepted: true}, nil
}
