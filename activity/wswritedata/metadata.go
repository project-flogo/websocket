package wswritedata

// Settings are the settings for the websocket proxy
type Settings struct {
}

// Input is the input into the websocket proxy
type Input struct {
	WSConnection interface{} `md:"wsconnection, required"`
	Message      interface{} `md:"message,required"`
}

// ToMap converts the input into a map
func (i *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"message":      i.Message,
		"wsconnection": i.WSConnection,
	}
}

// FromMap converts the values from a map to a struct
func (i *Input) FromMap(values map[string]interface{}) (err error) {
	i.Message = values["message"]
	i.WSConnection = values["wsconnection"]
	return nil
}

// Output is the output of the websocket proxy
type Output struct {
}
