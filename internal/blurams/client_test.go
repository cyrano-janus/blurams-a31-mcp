package blurams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoginAndCameraMapping(t *testing.T) {
	var loginBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/user/login":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(body)
			loginBody = string(encoded)
			_, _ = w.Write([]byte(`{"code":200,"data":{"userToken":"session-token"}}`))
		case "/sclient/compatible/device/list":
			if r.Header.Get("userToken") != "session-token" {
				t.Errorf("missing session token")
			}
			_, _ = w.Write([]byte(`{"code":200,"data":{"deviceList":[{"deviceId":"cam-1","deviceName":"Wohnzimmer","cameraModelId":"A31","onlineStatus":"available","comment":"{'FEATURE': 48}"},{"deviceName":"invalid"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("a@example.com", "secret", Options{
		APIHost: server.URL, RelayURL: "wss://relay.invalid",
		Now: func() time.Time { return time.UnixMilli(123) },
	})
	cameras, err := client.ListCameras(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cameras) != 1 || cameras[0].Model != "A31" || !cameras[0].Online || !cameras[0].SupportsPan || !cameras[0].SupportsTilt {
		t.Fatalf("cameras = %+v", cameras)
	}
	if strings.Contains(loginBody, `"password":"secret"`) {
		t.Fatal("plain password was sent")
	}
	if !strings.Contains(loginBody, `"password":"36cbca07d1617c974edd8c1bc978696c"`) {
		t.Fatalf("unexpected login body: %s", loginBody)
	}
}

func TestVendorErrorIsReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":401,"msg":"bad login","data":{}}`))
	}))
	defer server.Close()
	client := NewClient("a@example.com", "wrong", Options{APIHost: server.URL})
	_, err := client.ListCameras(context.Background())
	if err == nil || err.Error() != "bad login" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoginRequiresToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{}}`))
	}))
	defer server.Close()
	client := NewClient("a@example.com", "secret", Options{APIHost: server.URL})
	_, err := client.ListCameras(context.Background())
	if err == nil || err.Error() != "login response contained no usable token" {
		t.Fatalf("error = %v", err)
	}
}

func TestMoveValidatesBeforeNetwork(t *testing.T) {
	client := NewClient("a@example.com", "secret", Options{})
	camera := Camera{ID: "cam-1", ProductKey: "pk"}
	if _, err := client.Move(context.Background(), camera, "diagonal", 1); err == nil {
		t.Fatal("expected direction error")
	}
	if _, err := client.Move(context.Background(), camera, "left", 6); err == nil {
		t.Fatal("expected steps error")
	}
}

func TestHeadersNeverContainPassword(t *testing.T) {
	client := NewClient("a@example.com", "secret", Options{Now: func() time.Time { return time.UnixMilli(123) }})
	headers := client.headers("/path", "{}", "session-token")
	encoded, _ := json.Marshal(headers)
	if headers["Accept-Time"] != "123" || headers["userToken"] != "session-token" || strings.Contains(string(encoded), "secret") {
		t.Fatalf("headers = %s", encoded)
	}
}
