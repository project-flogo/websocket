package wsserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/project-flogo/core/action"
	"github.com/project-flogo/core/data/metadata"
	"github.com/project-flogo/core/support/log"
	"github.com/project-flogo/core/trigger"
)

var triggerMd = trigger.NewMetadata(&Settings{}, &Output{}, &HandlerSettings{})

func init() {
	trigger.Register(&Trigger{}, &Factory{})
}

type Factory struct {
}

// Metadata implements trigger.Factory.Metadata
func (*Factory) Metadata() *trigger.Metadata {
	return triggerMd
}

// Trigger REST trigger struct
type Trigger struct {
	server   *Server
	runner   action.Runner
	wsconn   *websocket.Conn
	settings *Settings
	logger   log.Logger
}

// New implements trigger.Factory.New
func (*Factory) New(config *trigger.Config) (trigger.Trigger, error) {
	s := &Settings{}
	err := metadata.MapToStruct(config.Settings, s, true)
	if err != nil {
		return nil, err
	}

	return &Trigger{settings: s}, nil
}

//Initialize
func (t *Trigger) Initialize(ctx trigger.InitContext) error {
	t.logger = ctx.Logger()
	router := httprouter.New()
	addr := ":" + strconv.Itoa(t.settings.Port)

	if t.settings == nil {
		panic(fmt.Sprintf("No Settings found for trigger"))
	}
	//Check whether TLS (Transport Layer Security) is enabled for the trigger
	enableTLS := false
	serverCert := ""
	serverKey := ""
	if t.settings.EnabledTLS != false {
		enableTLSSetting := t.settings.EnabledTLS
		if enableTLSSetting {
			//TLS is enabled, get server certificate & key
			enableTLS = true
			if t.settings.ServerCert == "" {
				panic(fmt.Sprintf("No serverCert found for trigger in settings"))
			}
			serverCert = t.settings.ServerCert

			if t.settings.ServerKey == "" {
				panic(fmt.Sprintf("No serverKey found for trigger in settings"))
			}
			serverKey = t.settings.ServerKey
		}
	}
	//Check whether client auth is enabled
	enableClientAuth := false
	trustStore := ""
	if t.settings.ClientAuthEnabled != false {
		enableClientAuthSetting := t.settings.ClientAuthEnabled
		if enableClientAuthSetting {
			enableClientAuth = true
			if t.settings.TrustStore == "" {
				panic(fmt.Sprintf("Client auth is enabled but client trust store is not provided for trigger in settings"))
			}
			trustStore = t.settings.TrustStore
		}
	}

	// Init handlers
	for _, handler := range ctx.GetHandlers() {

		s := &HandlerSettings{}
		err := metadata.MapToStruct(handler.Settings(), s, true)
		if err != nil {
			return err
		}

		method := s.Method
		path := s.Path
		mode := s.Mode

		router.Handle(method, path, newActionHandler(t, handler, mode))
	}

	t.logger.Debugf("Configured on port %d", t.settings.Port)
	t.server = NewServer(addr, router, enableTLS, serverCert, serverKey, enableClientAuth, trustStore)

	return nil
}

func (t *Trigger) Start() error {
	return t.server.Start()
}

func (t *Trigger) Stop() error {
	t.wsconn.Close()
	return t.server.Stop()
}

func newActionHandler(rt *Trigger, handler trigger.Handler, mode string) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rt.logger.Infof("received incomming request")

		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		rt.wsconn = conn
		if err != nil {
			rt.logger.Errorf("upgrade error", err)
		} else {
			//upgraded to websocket connection
			clientAdd := conn.RemoteAddr()
			rt.logger.Infof("Upgraded to websocket protocol")
			rt.logger.Infof("Remote address:", clientAdd)
			if mode == "1" {
				defer conn.Close()
				for {
					_, message, err := rt.wsconn.ReadMessage()
					if err != nil {
						rt.logger.Errorf("error while reading websocket message: %s", err)
						break
					}
					handlerRoutine(message, handler)
				}
				rt.logger.Infof("stopped listening to websocket endpoint")
			}
			if mode == "2" {
				out := &Output{
					QueryParams:  make(map[string]string),
					PathParams:   make(map[string]string),
					Headers:      make(map[string]string),
					WSconnection: conn,
				}
				_, err := handler.Handle(context.Background(), out)
				if err != nil {
					rt.logger.Errorf("Run action  failed [%s] ", err)
				}
				rt.logger.Infof("stopped listening to websocket endpoint")
			}
		}

	}
}

func handlerRoutine(message []byte, handler trigger.Handler) error {
	var content interface{}
	json.NewDecoder(bytes.NewBuffer(message)).Decode(&content)
	out := &Output{}
	out.Content = content
	_, err := handler.Handle(context.Background(), out)
	if err != nil {
		return fmt.Errorf("Run action  failed [%s] ", err)
	}
	return nil
}
