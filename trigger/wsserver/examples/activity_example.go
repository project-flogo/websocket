package examples

import (
	"github.com/project-flogo/contrib/activity/rest"
	"github.com/project-flogo/core/api"
	"github.com/project-flogo/core/engine"
	"github.com/project-flogo/microgateway"
	microapi "github.com/project-flogo/microgateway/api"
	"github.com/project-flogo/websocket/activity/wsproxy"
	trigger "github.com/project-flogo/websocket/trigger/wsserver"
)

// Example returns an API example
func Example(mode string, maxconn string) (engine.Engine, error) {
	app := api.NewApp()

	gateway := microapi.New("WSProxy")

	serviceWS := gateway.NewService("WSProxy", &wsproxy.Activity{})
	serviceWS.SetDescription("Websocket Activity service")
	serviceWS.AddSetting("uri", "ws://localhost:8080/ws")
	serviceWS.AddSetting("maxconnections", maxconn)

	serviceStore := gateway.NewService("PetStorePets", &rest.Activity{})
	serviceStore.SetDescription("Initiate action to get Petsotre details")
	serviceStore.AddSetting("uri", "http://localhost:8080/pets")
	serviceStore.AddSetting("method", "PUT")

	if mode == "2" {
		step := gateway.NewStep(serviceWS)
		step.AddInput("wsconnection", "=$.payload.wsconnection")
	}

	if mode == "1" {
		step := gateway.NewStep(serviceStore)
		step.AddInput("content", "=$.payload.content")
	}

	settings, err := gateway.AddResource(app)
	if err != nil {
		return nil, err
	}

	trg := app.NewTrigger(&trigger.Trigger{}, &trigger.Settings{
		Port:              9096,
		EnabledTLS:        false,
		ServerCert:        "",
		ServerKey:         "",
		ClientAuthEnabled: false,
		TrustStore:        "",
	})
	handler, err := trg.NewHandler(&trigger.HandlerSettings{
		Method: "GET",
		Path:   "/ws",
		Mode:   mode,
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
