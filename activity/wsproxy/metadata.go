package wsproxy

// Settings are the settings for the websocket proxy
type Settings struct {
	URI            string `md:"uri,required"`
	MaxConnections string `md:"maxconnections"`
}

// Input is the input into the websocket proxy
type Input struct {
	WSconnection interface{} `md:"wsconnection,required"`
}

// ToMap converts the input into a map
func (o *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"wsconnection": o.WSconnection,
	}
}

// FromMap converts the values from a map to a struct
func (o *Input) FromMap(values map[string]interface{}) error {
	o.WSconnection = values["wsconnection"]
	return nil
}

// Output is the output of the websocket proxy
type Output struct {
}
