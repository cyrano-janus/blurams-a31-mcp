package mcpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cyrano-janus/blurams-a31-mcp/internal/blurams"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CameraService interface {
	ListCameras(context.Context) ([]blurams.Camera, error)
	Move(context.Context, blurams.Camera, string, int) (blurams.MoveResult, error)
}

type Service struct {
	client   CameraService
	cameraID string
}

type listInput struct{}

type listOutput struct {
	Cameras []blurams.Camera `json:"cameras"`
}

type cameraInput struct {
	CameraID string `json:"camera_id,omitempty" jsonschema:"optional camera ID; required when the account has multiple cameras"`
}

type cameraOutput struct {
	Camera blurams.Camera `json:"camera"`
}

type moveInput struct {
	Direction string `json:"direction" jsonschema:"movement direction: home, left, right, up, or down"`
	CameraID  string `json:"camera_id,omitempty" jsonschema:"optional camera ID; required when the account has multiple cameras"`
	Steps     *int   `json:"steps,omitempty" jsonschema:"number of small movement steps from 1 to 5; defaults to 1"`
}

func New(client CameraService, cameraID string) *mcp.Server {
	svc := &Service{client: client, cameraID: cameraID}
	server := mcp.NewServer(&mcp.Implementation{Name: "blurams-a31-mcp", Version: "0.2.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name: "list_cameras", Description: "List Blurams cameras and their current online and pan/tilt status.",
	}, svc.listCameras)
	mcp.AddTool(server, &mcp.Tool{
		Name: "camera_status", Description: "Return current status for one configured Blurams camera.",
	}, svc.cameraStatus)
	mcp.AddTool(server, &mcp.Tool{
		Name: "move_camera", Description: "Move one Blurams camera by 1-5 small steps: home, left, right, up, or down.",
	}, svc.moveCamera)
	return server
}

func (s *Service) listCameras(ctx context.Context, _ *mcp.CallToolRequest, _ listInput) (*mcp.CallToolResult, listOutput, error) {
	cameras, err := s.client.ListCameras(ctx)
	if err != nil {
		return nil, listOutput{}, err
	}
	return nil, listOutput{Cameras: cameras}, nil
}

func (s *Service) cameraStatus(ctx context.Context, _ *mcp.CallToolRequest, input cameraInput) (*mcp.CallToolResult, cameraOutput, error) {
	camera, err := s.selectedCamera(ctx, input.CameraID)
	if err != nil {
		return nil, cameraOutput{}, err
	}
	return nil, cameraOutput{Camera: camera}, nil
}

func (s *Service) moveCamera(ctx context.Context, _ *mcp.CallToolRequest, input moveInput) (*mcp.CallToolResult, blurams.MoveResult, error) {
	steps := 1
	if input.Steps != nil {
		steps = *input.Steps
	}
	direction := strings.ToLower(input.Direction)
	if steps < 1 || steps > 5 {
		return nil, blurams.MoveResult{}, errors.New("steps must be between 1 and 5")
	}
	switch direction {
	case "home", "left", "right", "up", "down":
	default:
		return nil, blurams.MoveResult{}, errors.New("invalid direction")
	}
	camera, err := s.selectedCamera(ctx, input.CameraID)
	if err != nil {
		return nil, blurams.MoveResult{}, err
	}
	switch direction {
	case "left", "right", "LEFT", "RIGHT":
		if !camera.SupportsPan {
			return nil, blurams.MoveResult{}, errors.New("camera does not report pan support")
		}
	case "up", "down", "UP", "DOWN":
		if !camera.SupportsTilt {
			return nil, blurams.MoveResult{}, errors.New("camera does not report tilt support")
		}
	}
	result, err := s.client.Move(ctx, camera, direction, steps)
	if err != nil {
		return nil, blurams.MoveResult{}, err
	}
	return nil, result, nil
}

func (s *Service) selectedCamera(ctx context.Context, requested string) (blurams.Camera, error) {
	if s.cameraID != "" && requested != "" && requested != s.cameraID {
		return blurams.Camera{}, errors.New("camera_id is outside configured access")
	}
	cameras, err := s.client.ListCameras(ctx)
	if err != nil {
		return blurams.Camera{}, err
	}
	id := requested
	if id == "" {
		id = s.cameraID
	}
	if id != "" {
		for _, camera := range cameras {
			if camera.ID == id {
				return camera, nil
			}
		}
		return blurams.Camera{}, errors.New("configured camera was not returned by Blurams")
	}
	if len(cameras) != 1 {
		return blurams.Camera{}, errors.New("specify camera_id because the account does not contain exactly one camera")
	}
	return cameras[0], nil
}

func BearerMiddleware(expected string) func(http.Handler) http.Handler {
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if len(expected) < 32 || len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			return nil, fmt.Errorf("%w: bearer token rejected", auth.ErrInvalidToken)
		}
		return &auth.TokenInfo{
			Scopes: []string{"camera:control"}, Expiration: time.Now().Add(5 * time.Minute), UserID: "owner",
		}, nil
	}
	return auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{Scopes: []string{"camera:control"}})
}
