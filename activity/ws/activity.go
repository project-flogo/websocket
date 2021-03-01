package ws

import (
	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/activity"
	"github.com/project-flogo/core/data/metadata"
)

func init() {
	activity.Register(&Activity{}, New)
}

var activityMd = activity.ToMetadata(&Settings{}, &Input{}, &Output{})

// New create a new websocket client
func New(ctx activity.InitContext) (activity.Activity, error) {
	s := &Settings{}
	err := metadata.MapToStruct(ctx.Settings(), s, true)
	if err != nil {
		return nil, err
	}

	connection, _, err := websocket.DefaultDialer.Dial(s.URI, nil)
	if err != nil {
		return nil, err
	}

	act := &Activity{
		settings:   s,
		connection: connection,
	}
	return act, nil
}

// Activity is an activity that is used to invoke a Web socket operation
type Activity struct {
	settings   *Settings
	connection *websocket.Conn
}

// Metadata returns the metadata for a websocket client
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Invokes a web socket operation
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {
	input := &Input{}
	ctx.GetInputObject(input)

	var message []byte
	if input.Message != nil {
		if value, ok := input.Message.(string); ok {
			message = []byte(value)
		} else {
			value, err := json.Marshal(input.Message)
			if err != nil {
				return false, err
			}
			message = value
		}
	}

	err = a.connection.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		return false, err
	}

	return true, nil
}
