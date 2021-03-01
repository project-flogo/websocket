package wsclient

// Settings for the websocket client trigger
type Settings struct {
	URL string `md:"url,required"`
}

// Output is the outputs for the websocket trigger
type Output struct {
	Content interface{} `md:"content"`
}

// ToMap converts the output to a map
func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"content": o.Content,
	}
}

// FromMap converts the values from a map to a struct
func (o *Output) FromMap(values map[string]interface{}) error {
	o.Content = values["content"]
	return nil
}
