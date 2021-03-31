package wsserver

import "github.com/project-flogo/core/data/coerce"

// Settings are the settings for the websocket server
type Settings struct {
	Port              int    `md:"port,required"`
	EnabledTLS        bool   `md:"enableTLS"`
	ServerCert        string `md:"serverCert"`
	ServerKey         string `md:"serverKey"`
	ClientAuthEnabled bool   `md:"enableClientAuth"`
	TrustStore        string `md:"trustStore"`
}

// Output are the outputs of the websocket server
type Output struct {
	PathParams   map[string]string `md:"pathParams"`
	QueryParams  map[string]interface{} `md:"queryParams"`
	Headers      map[string]interface{} `md:"headers"`
	Content      interface{}       `md:"content"`
	WSconnection interface{}       `md:"wsconnection"`
}

// ToMap converts the output struct to a map
func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"pathParams":   o.PathParams,
		"queryParams":  o.QueryParams,
		"headers":      o.Headers,
		"content":      o.Content,
		"wsconnection": o.WSconnection,
	}
}

// FromMap converts the output from a map
func (o *Output) FromMap(values map[string]interface{}) (err error) {
	o.PathParams, err = coerce.ToParams(values["pathParams"])
	if err != nil {
		return err
	}
	o.QueryParams, err = coerce.ToObject(values["queryParams"])
	if err != nil {
		return err
	}
	o.Content = values["content"]
	o.Headers, err = coerce.ToObject(values["headers"])
	if err != nil {
		return err
	}
	o.WSconnection = values["wsconnection"]
	return nil
}

// HandlerSettings are the settings for a handler
type HandlerSettings struct {
	Method string `md:"method,required,allowed(GET,POST,PUT,PATCH,DELETE)"`
	Path   string `md:"path,required"`
	Mode   string `md:"mode,required"`
}
