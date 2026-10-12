package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cyrano-janus/blurams-a31-mcp/internal/blurams"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeClient struct {
	cameras []blurams.Camera
	moves   []blurams.MoveResult
}

func TestStreamableHTTPWithOfficialClient(t *testing.T) {
	server := New(&fakeClient{cameras: []blurams.Camera{testCamera("cam-1")}}, "cam-1")
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	token := strings.Repeat("a", 64)
	endpoint := httptest.NewServer(BearerMiddleware(token)(handler))
	defer endpoint.Close()
	httpClient := &http.Client{Transport: tokenTransport{token: token}}
	client := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint.URL, HTTPClient: httpClient}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("tools=%+v error=%v", tools, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "camera_status", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

type tokenTransport struct{ token string }

func (t tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func (f *fakeClient) ListCameras(context.Context) ([]blurams.Camera, error) { return f.cameras, nil }
func (f *fakeClient) Move(_ context.Context, camera blurams.Camera, direction string, steps int) (blurams.MoveResult, error) {
	result := blurams.MoveResult{CameraID: camera.ID, Direction: direction, Steps: steps, Accepted: true}
	f.moves = append(f.moves, result)
	return result, nil
}

func testCamera(id string) blurams.Camera {
	return blurams.Camera{ID: id, Name: "Wohnzimmer", Model: "A31", Online: true, SupportsPan: true, SupportsTilt: true, ProductKey: "must-not-leak"}
}

func connect(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func TestToolsRegisteredAndProductKeyRedacted(t *testing.T) {
	client := connect(t, New(&fakeClient{cameras: []blurams.Camera{testCamera("cam-1")}}, ""))
	tools, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 3 {
		t.Fatalf("tool count = %d", len(tools.Tools))
	}
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_cameras", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || contains(string(encoded), "product_key") || contains(string(encoded), "must-not-leak") {
		t.Fatalf("sensitive field leaked: %s", encoded)
	}
}

func TestMoveUsesConfiguredCameraAndDefaultStep(t *testing.T) {
	fake := &fakeClient{cameras: []blurams.Camera{testCamera("cam-1")}}
	client := connect(t, New(fake, "cam-1"))
	_, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "move_camera", Arguments: map[string]any{"direction": "left"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.moves) != 1 || fake.moves[0].CameraID != "cam-1" || fake.moves[0].Steps != 1 {
		t.Fatalf("moves = %+v", fake.moves)
	}
}

func TestAmbiguousAccountFailsClosed(t *testing.T) {
	client := connect(t, New(&fakeClient{cameras: []blurams.Camera{testCamera("one"), testCamera("two")}}, ""))
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "camera_status", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected MCP tool error")
	}
}

func TestBearerMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := BearerMiddleware("01234567890123456789012345678901")(next)

	for _, tc := range []struct {
		name, token string
		status      int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "wrong", http.StatusUnauthorized},
		{"valid", "01234567890123456789012345678901", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func contains(value, substring string) bool {
	for i := 0; i+len(substring) <= len(value); i++ {
		if value[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}

func TestMovementGuards(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       map[string]any
		configured string
		pan        bool
	}{
		{"zero", map[string]any{"direction": "left", "steps": 0}, "", true},
		{"excess", map[string]any{"direction": "left", "steps": 6}, "", true},
		{"invalid", map[string]any{"direction": "diagonal"}, "", true},
		{"override", map[string]any{"direction": "left", "camera_id": "other"}, "cam-1", true},
		{"mixed-case-pan", map[string]any{"direction": "Left"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			camera := testCamera("cam-1")
			camera.SupportsPan = tc.pan
			fake := &fakeClient{cameras: []blurams.Camera{camera, testCamera("other")}}
			if tc.configured == "" {
				fake.cameras = fake.cameras[:1]
			}
			session := connect(t, New(fake, tc.configured))
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "move_camera", Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || len(fake.moves) != 0 {
				t.Fatalf("guard failed: %+v", result)
			}
		})
	}
}

func TestShortServerTokenFailsClosed(t *testing.T) {
	handler := BearerMiddleware("short")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer short")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMixedCaseDirectionIsNormalized(t *testing.T) {
	fake := &fakeClient{cameras: []blurams.Camera{testCamera("cam-1")}}
	session := connect(t, New(fake, "cam-1"))
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "move_camera", Arguments: map[string]any{"direction": "Left", "steps": 5}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if len(fake.moves) != 1 || fake.moves[0].Direction != "left" || fake.moves[0].Steps != 5 {
		t.Fatalf("moves=%+v", fake.moves)
	}
}
