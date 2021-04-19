package wswritedata

import (
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/project-flogo/core/activity"
	"github.com/project-flogo/core/data/coerce"
)

func init() {
	activity.Register(&Activity{}, New)
}

var activityMd = activity.ToMetadata(&Settings{}, &Input{}, &Output{})

// New create a new websocket client
func New(ctx activity.InitContext) (activity.Activity, error) {
	act := &Activity{}
	return act, nil
}

// Activity is an activity that is used to invoke a Web socket operation
type Activity struct{}

// Metadata returns the metadata for a websocket client
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Invokes a web socket operation
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {
	input := &Input{}
	err = ctx.GetInputObject(input)
	if err != nil {
		return false, err
	}
	logger := ctx.Logger()
	if input.WSConnection == nil {
		return false, errors.New("WSConnection is not configured")
	}
	conn, ok := input.WSConnection.(*websocket.Conn)
	if !ok {
		return false, errors.New("Configured connection is not a WebSocket Connection")
	}
	//populate msg
	if input.Message != nil {
		message, err := coerce.ToBytes(input.Message)
		if err != nil {
			return false, err
		}
		logger.Info("writing data to websocket connection")
		err = conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			logger.Errorf("Error while writing to websocket connection - %v", err)
			return false, err
		}
	} else {
		return false, errors.New("Message is not configured")
	}
	return true, nil
}
