package examples

import (
	trigger "github.com/project-flogo/contrib/trigger/rest"
	"github.com/project-flogo/core/api"
	"github.com/project-flogo/core/engine"
	"github.com/project-flogo/microgateway"
	microapi "github.com/project-flogo/microgateway/api"
	"github.com/project-flogo/websocket/activity/ws"
)

// Example returns a ws example
func Example() (engine.Engine, error) {
	app := api.NewApp()

	gateway := microapi.New("Websocket")
	service := gateway.NewService("Websocket", &ws.Activity{})
	service.SetDescription("Send a websocket message")
	service.AddSetting("uri", "ws://localhost:8080/ws")
	step := gateway.NewStep(service)
	step.AddInput("message", "=$.payload.content")
	response := gateway.NewResponse(false)
	response.SetCode(200)
	settings, err := gateway.AddResource(app)
	if err != nil {
		panic(err)
	}

	trg := app.NewTrigger(&trigger.Trigger{}, &trigger.Settings{Port: 9096})
	handler, err := trg.NewHandler(&trigger.HandlerSettings{
		Method: "POST",
		Path:   "/message",
	})
	if err != nil {
		return nil, err
	}

	_, err = handler.NewAction(&microgateway.Action{}, settings)
	if err != nil {
		return nil, err
	}

	return api.NewEngine(app)
}
