package ws

import "github.com/project-flogo/core/data/coerce"

// Settings are the settings for the websocket proxy
type Settings struct {
	URI           string `md:"uri,required"`
	AllowInsecure bool   `md:"allowInsecure"`
	CaCert        string `md:"caCert"`
}

// Input is the input into the websocket proxy
type Input struct {
	Message     interface{}            `md:"message,required"`
	PathParams  map[string]string      `md:"pathParams"`
	QueryParams map[string]interface{} `md:"queryParams"`
	Headers     map[string]interface{} `md:"headers"`
}

// ToMap converts the input into a map
func (i *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"message":     i.Message,
		"pathParams":  i.PathParams,
		"queryParams": i.QueryParams,
		"headers":     i.Headers,
	}
}

// FromMap converts the values from a map to a struct
func (i *Input) FromMap(values map[string]interface{}) error {
	i.Message = values["message"]
	i.PathParams, err = coerce.ToParams(values["pathParams"])
	if err != nil {
		return err
	}
	i.QueryParams, err = coerce.ToObject(values["queryParams"])
	if err != nil {
		return err
	}
	i.Headers, err = coerce.ToObject(values["headers"])
	if err != nil {
		return err
	}
	return nil
}

// Output is the output of the websocket proxy
type Output struct {
}
