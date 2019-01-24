package wsclient

type Settings struct {
	URL string `md:"url,required"`
}

type Output struct {
	Content interface{} `md:"content"`
}

func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"content": o.Content,
	}
}

func (o *Output) FromMap(values map[string]interface{}) error {
	o.Content = values["content"]
	return nil
}
