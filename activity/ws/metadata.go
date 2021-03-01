package ws

// Settings are the settings for the websocket proxy
type Settings struct {
	URI string `md:"uri,required"`
}

// Input is the input into the websocket proxy
type Input struct {
	Message interface{} `md:"message"`
}

// ToMap converts the input into a map
func (o *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"message": o.Message,
	}
}

// FromMap converts the values from a map to a struct
func (o *Input) FromMap(values map[string]interface{}) error {
	o.Message = values["message"]
	return nil
}

// Output is the output of the websocket proxy
type Output struct {
}
