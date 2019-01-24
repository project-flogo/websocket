package wsserver

type Settings struct {
	Port              int    `md:"port,required"`
	EnabledTLS        bool   `md:"enableTLS"`
	ServerCert        string `md:"serverCert"`
	ServerKey         string `md:"serverKey"`
	ClientAuthEnabled bool   `md:"enableClientAuth"`
	TrustStore        string `md:"trustStore"`
}

type Output struct {
	PathParams   map[string]string `md:"pathParams"`
	QueryParams  map[string]string `md:"queryParams"`
	Headers      map[string]string `md:"headers"`
	Content      interface{}       `md:"content"`
	WSconnection interface{}       `md:"wsconnection"`
}

func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"pathParams":   o.PathParams,
		"queryParams":  o.QueryParams,
		"headers":      o.Headers,
		"content":      o.Content,
		"wsconnection": o.WSconnection,
	}
}

func (o *Output) FromMap(values map[string]interface{}) error {
	o.PathParams = values["pathParams"].(map[string]string)
	o.QueryParams = values["queryParams"].(map[string]string)
	o.Content = values["content"]
	o.Headers = values["headers"].(map[string]string)
	o.WSconnection = values["wsconnection"]
	return nil
}

type HandlerSettings struct {
	Method string `md:"method,required,allowed(GET,POST,PUT,PATCH,DELETE)"`
	Path   string `md:"path,required"`
	Mode   string `md:"mode,required"`
}
