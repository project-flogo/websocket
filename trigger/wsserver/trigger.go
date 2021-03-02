package wsserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/project-flogo/core/action"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/metadata"
	"github.com/project-flogo/core/support/log"
	"github.com/project-flogo/core/trigger"
)

var triggerMd = trigger.NewMetadata(&Settings{}, &Output{}, &HandlerSettings{})

const (
	// ModeMessage sends messages to the action
	ModeMessage = "Data"
	// ModeConnection sends connections to the action
	ModeConnection = "Connection"
)

func init() {
	trigger.Register(&Trigger{}, &Factory{})
}

// Factory is a factory for websocket servers
type Factory struct {
}

// Metadata implements trigger.Factory.Metadata
func (*Factory) Metadata() *trigger.Metadata {
	return triggerMd
}

// Trigger trigger struct
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

// Initialize initializes triggers
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
		router.Handle(method, replacePath(path), newActionHandler(t, handler, mode))
	}

	t.logger.Debugf("Configured on port %d", t.settings.Port)
	t.server = NewServer(addr, router, enableTLS, serverCert, serverKey, enableClientAuth, trustStore)

	return nil
}

// Start starts the trigger
func (t *Trigger) Start() error {
	return t.server.Start()
}

// Stop stops the trigger
func (t *Trigger) Stop() error {
	t.wsconn.Close()
	return t.server.Stop()
}

func replacePath(path string) string {
	path = strings.Replace(path, "}", "", -1)
	return strings.Replace(path, "{", ":", -1)
}

func newActionHandler(rt *Trigger, handler trigger.Handler, mode string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rt.logger.Infof("received incomming request")
		out := &Output{
			QueryParams: make(map[string]interface{}),
			PathParams:  make(map[string]string),
			Headers:     make(map[string]interface{}),
		}

		// populate other params
		outconfigured, err := coerce.ToObject(handler.Schemas().Output)
		if err != nil {
			rt.logger.Errorf("Unable to parse Output Object", err)
			return
		}

		//PathParams
		if len(ps) > 0 {
			pathParamMetadata, _ := outconfigured["pathParams"]
			if pathParamMetadata != nil {
				resultWithPathparams, err := ParseOutputPathParams(pathParamMetadata, ps, rt)
				if err != nil {
					rt.logger.Info("Unable to parse Path Params: ", err)
					//return
				} else if resultWithPathparams != nil {
					out.PathParams = resultWithPathparams
				}
			}
		}
		//QueryParams
		queryParamMetadata, _ := outconfigured["queryParams"]
		if queryParamMetadata != nil {
			resultWithQueryparams, err := ParseOutputQueryParams(queryParamMetadata, r, w, rt)
			if err != nil {
				return
			} else if resultWithQueryparams != nil {
				out.QueryParams = resultWithQueryparams
			}
		}
		//Headers
		headerMetadata, _ := outconfigured["headers"]
		if headerMetadata != nil {
			resultWithHeaders, err := ParseOutputHeaders(headerMetadata, r, w, rt)
			if err != nil {
				//rt.logger.Info("Unable to parse Headers: ", err)
				return
			} else if resultWithHeaders != nil {
				out.Headers = resultWithHeaders
			}
		}

		// upgrade conn
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			rt.logger.Errorf("upgrade error", err)
			return
		}
		rt.wsconn = conn
		clientAdd := conn.RemoteAddr()
		rt.logger.Infof("Upgraded to websocket protocol")
		rt.logger.Infof("Remote address:", clientAdd)

		// params
		switch mode {
		case ModeMessage:
			defer conn.Close()
			for {
				_, message, err := rt.wsconn.ReadMessage()
				if err != nil {
					rt.logger.Infof("error while reading websocket message: %s", err)
					break
				}
				handlerRoutine(message, handler, out)
			}
			rt.logger.Infof("stopped listening to websocket endpoint")
		case ModeConnection:
			out.WSconnection = conn
			_, err := handler.Handle(context.Background(), out)
			if err != nil {
				rt.logger.Errorf("Run action  failed [%s] ", err)
			}
			rt.logger.Infof("stopped listening to websocket endpoint")
		}
	}
}

func handlerRoutine(message []byte, handler trigger.Handler, out *Output) error {
	var content interface{}
	if (handler.Settings()["format"] != nil && handler.Settings()["format"].(string) == "JSON") ||
		(handler.Settings()["format"] == nil && isJSON(message)) {
		json.NewDecoder(bytes.NewBuffer(message)).Decode(&content)
	} else {
		content = string(message)
	}
	out.Content = content
	_, err := handler.Handle(context.Background(), out)
	if err != nil {
		return fmt.Errorf("Run action  failed [%s] ", err)
	}
	return nil
}

func isJSON(str []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(str, &js) == nil
}
func getValuewithType(param Parameter, sv []string) ([]interface{}, error) {
	var values []interface{}
	switch param.Type { // json schema data type
	case "number":
		if param.Repeating == "false" {
			v, err := strconv.ParseFloat(sv[0], 64)
			if err != nil {
				return nil, fmt.Errorf("value %s is not a %s type", sv[0], param.Type)
			}
			values = append(values, v)
		} else {
			for _, item := range sv {
				v, err := strconv.ParseFloat(item, 64)
				if err != nil {
					return nil, fmt.Errorf("value %s is not a %s type", item, param.Type)
				}
				values = append(values, v)
			}
		}

	case "integer":
		if param.Repeating == "false" {
			v, err := strconv.ParseInt(sv[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("value %s is not a %s type", sv[0], param.Type)
			}
			values = append(values, v)
		} else {
			for _, item := range sv {
				v, err := strconv.ParseInt(item, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("value %s is not a %s type", item, param.Type)
				}
				values = append(values, v)
			}
		}

	case "boolean":
		if param.Repeating == "false" {
			v, err := strconv.ParseBool(sv[0])
			if err != nil {
				return nil, fmt.Errorf("value %s is not a %s type", sv[0], param.Type)
			}
			values = append(values, v)

		} else {
			for _, item := range sv {
				v, err := strconv.ParseBool(item)
				if err != nil {
					return nil, fmt.Errorf("value %s is not a %s type", item, param.Type)
				}
				values = append(values, v)
			}
		}
	case "string":
		if param.Repeating == "false" {
			v, err := coerce.ToString(sv[0])
			if err != nil {
				return nil, err
			}
			values = append(values, v)

		} else {
			for _, item := range sv {
				v, err := coerce.ToString(item)
				if err != nil {
					return nil, err
				}
				values = append(values, v)
			}
		}
	}
	return values, nil
}

func ParseTillValue(outputJsonData interface{}) (map[string]interface{}, error) {
	casted, err := coerce.ToObject(outputJsonData)
	if err != nil {
		return nil, err
	}
	castedValue := casted["value"]
	if castedValue != nil {
		sec, err := coerce.ToObject(castedValue)
		if err != nil {
			return nil, err
		}
		return sec, nil
	}
	return nil, nil
}
func ParseOutputPathParams(outputJsonData interface{}, ps httprouter.Params, rt *Trigger) (map[string]string, error) {
	/*for key, val := range outputJsonData.(map[string]interface{}){
		fmt.Println("*****key is :", key, " *****value is : ", val)
	}*/
	sec, err := ParseTillValue(outputJsonData)
	if err != nil {
		rt.logger.Info("Unable to convert table value data to object", err)
		return nil, nil
	}
	if sec != nil {
		definePathParam, _ := ParseParams(sec)
		fmt.Println("definePathParam is : ", definePathParam)
		if definePathParam != nil {
			pathParams := make(map[string]string)
			for _, qParam := range definePathParam {
				if ps.ByName(qParam.Name) == "" && strings.EqualFold(qParam.Required, "true") {
					errMsg := fmt.Sprintf("Required path parameter [%s] is not set", qParam.Name)
					rt.logger.Info(errMsg)
					//rt.logger.Error(errMsg)
					return nil, nil
				}
				if ps.ByName(qParam.Name) != "" {
					values, err := getValuewithType(qParam, []string{ps.ByName(qParam.Name)})
					if err != nil {
						errMsg := fmt.Sprintf("Fail to validate path parameter: %v", err)
						rt.logger.Info(errMsg)
						//rt.logger.Error(errMsg)
						return nil, nil
					}
					pathParams[qParam.Name] = values[0].(string)
				}
			}
			return pathParams, nil
		}
	}
	return nil, nil
}

func ParseOutputQueryParams(outputJsonData interface{}, r *http.Request, w http.ResponseWriter, rt *Trigger) (map[string]interface{}, error) {
	/*for key, val := range outputJsonData.(map[string]interface{}){
		fmt.Println("*****query params key is :", key, " *****value is : ", val)
	}*/
	sec, err := ParseTillValue(outputJsonData)
	if err != nil {
		rt.logger.Info("Unable to convert table value data to object", err)
		return nil, nil
	}
	if sec != nil {
		definedQueryParams, _ := ParseParams(sec)
		if definedQueryParams != nil {
			queryValues := r.URL.Query()
			queryParams := make(map[string]interface{}, len(definedQueryParams))
			for _, qParam := range definedQueryParams {
				value := queryValues[qParam.Name]
				if !notEmpty(value) && strings.EqualFold(qParam.Required, "true") {
					errMsg := fmt.Sprintf("Required query parameter [%s] is not set", qParam.Name)
					rt.logger.Info(errMsg)
					http.Error(w, errMsg, http.StatusBadRequest)
					return nil, errors.New(errMsg)
				}

				if notEmpty(value) {
					values, err := getValuewithType(qParam, value)
					if err != nil {
						errMsg := fmt.Sprintf("Fail to validate query parameter: %v", err)
						rt.logger.Info(errMsg)
						http.Error(w, errMsg, http.StatusBadRequest)
						return nil, errors.New(errMsg)
					}
					if qParam.Repeating == "false" {
						queryParams[qParam.Name] = values[0]
					} else {
						queryParams[qParam.Name] = values
					}
					//rt.logger.Debugf("Query param: Name[%s], Value[%s]", qParam.Name, queryParams[qParam.Name])
				}
			}
			return queryParams, nil
		}
	}
	return nil, nil
}

func ParseOutputHeaders(outputJsonData interface{}, r *http.Request, w http.ResponseWriter, rt *Trigger) (map[string]interface{}, error) {
	sec, err := ParseTillValue(outputJsonData)
	if err != nil {
		rt.logger.Info("Unable to convert table value data to object", err)
		return nil, nil
	}
	if sec != nil {
		definedHeaderParams, _ := ParseParams(sec)
		if definedHeaderParams != nil {
			headers := make(map[string]interface{}, len(definedHeaderParams))
			headerValues := r.Header
			fmt.Println("&&&&&&&&&&& header values: ", headerValues)
			rt.logger.Debug(headerValues)
			for _, hParam := range definedHeaderParams {
				value := headerValues[http.CanonicalHeaderKey(hParam.Name)]
				if len(value) == 0 && hParam.Required == "true" {
					errMsg := fmt.Sprintf("Required header [%s] is not set", hParam.Name)
					rt.logger.Info(errMsg)
					http.Error(w, errMsg, http.StatusBadRequest)
					return nil, errors.New(errMsg)
				}
				if len(value) > 0 {
					values, err := getValuewithType(hParam, value)
					if err != nil {
						errMsg := fmt.Sprintf("Fail to validate header parameter: %v", err)
						rt.logger.Info(errMsg)
						http.Error(w, errMsg, http.StatusBadRequest)
						return nil, errors.New(errMsg)
					}
					if hParam.Repeating == "false" {
						headers[hParam.Name] = values[0]
					} else {
						headers[hParam.Name] = values
					}
					//rt.logger.Debugf("Header: Name[%s], Value[%s]", hParam.Name, headers[hParam.Name])
				}
			}
			return headers, nil
		}
	}
	return nil, nil
}

func notEmpty(array []string) bool {
	if len(array) > 0 {
		if len(array) == 1 {
			if array[0] != "" && len(array[0]) > 0 {
				return true
			}
			return false
		} else {
			return true
		}
	}
	return false
}
