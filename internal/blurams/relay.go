package blurams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func (c *Client) moveOnce(parent context.Context, token, cameraID, productKey string, value int) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	header := http.Header{}
	header.Set("Origin", "https://www.ehomeease.com")
	conn, _, err := websocket.Dial(ctx, c.relayURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return fmt.Errorf("connect relay: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	conn.SetReadLimit(1 << 20)

	nextCommand := 0
	send := func(message map[string]any) (int, error) {
		nextCommand++
		message["cmdId"] = nextCommand
		if err := wsjson.Write(ctx, conn, message); err != nil {
			return 0, fmt.Errorf("write relay message: %w", err)
		}
		return nextCommand, nil
	}
	if _, err := send(map[string]any{
		"type": 1, "token": token, "deviceId": fmt.Sprintf("WEBCLIENT_WEBSOCKET%d", c.now().UnixMilli()),
		"channelName": "websocket", "userName": "", "productKey": productKey,
	}); err != nil {
		return err
	}
	commandID, err := send(map[string]any{
		"type": 3, "cameraId": cameraID,
		"msg": map[string]any{
			"msgSession": 764963713, "msgSequence": 0, "msgTimeStamp": c.now().UnixMilli(),
			"msgCategory": "camera", "msgContent": map[string]any{
				"request": 1793, "requestParams": map[string]int{"value": value}, "subRequest": 5,
			},
		},
	})
	if err != nil {
		return err
	}

	for {
		var incoming struct {
			Type   int             `json:"type"`
			CmdID  int             `json:"cmdId"`
			Result *int            `json:"result"`
			Msg    json.RawMessage `json:"msg"`
		}
		if err := wsjson.Read(ctx, conn, &incoming); err != nil {
			return fmt.Errorf("read relay acknowledgement: %w", err)
		}
		if incoming.Type == 7 {
			if _, err := send(map[string]any{"type": 8}); err != nil {
				return err
			}
			continue
		}
		if incoming.Type == 100 && incoming.Result != nil && *incoming.Result != 0 {
			return errors.New("relay rejected the authentication token")
		}
		if (incoming.Type != 133 && incoming.Type != 134) || incoming.CmdID != commandID {
			continue
		}
		content, err := relayContent(incoming.Msg)
		if err != nil {
			return err
		}
		if content.ResponseRequest != 1793 || content.ResponseSubRequest != 5 {
			continue
		}
		if content.Response == nil || *content.Response != 0 {
			return errors.New("camera rejected the movement command")
		}
		return nil
	}
}

type responseContent struct {
	ResponseRequest    int  `json:"responseRequest"`
	ResponseSubRequest int  `json:"responseSubRequest"`
	Response           *int `json:"response"`
}

func relayContent(raw json.RawMessage) (responseContent, error) {
	if len(raw) == 0 {
		return responseContent{}, errors.New("relay acknowledgement contained no message")
	}
	if raw[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return responseContent{}, fmt.Errorf("decode relay message string: %w", err)
		}
		raw = []byte(encoded)
	}
	var message struct {
		Content responseContent `json:"msgContent"`
	}
	if err := json.Unmarshal(raw, &message); err != nil {
		return responseContent{}, fmt.Errorf("decode relay message: %w", err)
	}
	return message.Content, nil
}
