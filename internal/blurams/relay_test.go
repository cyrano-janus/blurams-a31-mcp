package blurams

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRelayContentAcceptsObjectAndEncodedString(t *testing.T) {
	payload := `{"msgContent":{"responseRequest":1793,"responseSubRequest":5,"response":0}}`
	for _, raw := range []json.RawMessage{json.RawMessage(payload), mustJSON(t, payload)} {
		content, err := relayContent(raw)
		if err != nil {
			t.Fatal(err)
		}
		if content.ResponseRequest != 1793 || content.ResponseSubRequest != 5 || content.Response == nil || *content.Response != 0 {
			t.Fatalf("content = %+v", content)
		}
	}
}

func TestWebsocketMovementAcknowledgement(t *testing.T) {
	for _, response := range []string{"0", "1", "null"} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"www.ehomeease.com"}})
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
				defer cancel()
				var auth, command map[string]any
				if err := wsjson.Read(ctx, conn, &auth); err != nil {
					t.Error(err)
					return
				}
				if auth["token"] != "token" {
					t.Error("missing token")
				}
				if err := wsjson.Read(ctx, conn, &command); err != nil {
					t.Error(err)
					return
				}
				if command["cameraId"] != "cam-1" {
					t.Error("wrong camera")
				}
				content := json.RawMessage(`{"msgContent":{"responseRequest":1793,"responseSubRequest":5,"response":` + response + `}}`)
				_ = wsjson.Write(ctx, conn, map[string]any{"type": 134, "cmdId": command["cmdId"], "msg": content})
			}))
			defer server.Close()
			c := NewClient("email", "password", Options{RelayURL: "ws" + strings.TrimPrefix(server.URL, "http")})
			c.token = "token"
			_, err := c.Move(context.Background(), Camera{ID: "cam-1", ProductKey: "pk"}, "right", 1)
			if (err == nil) != (response == "0") {
				t.Fatalf("response %s, error %v", response, err)
			}
		})
	}
}

func TestRelayContentRejectsMissingMessage(t *testing.T) {
	if _, err := relayContent(nil); err == nil {
		t.Fatal("expected error")
	}
}

func mustJSON(t *testing.T, value string) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
