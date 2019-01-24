package wsproxy

type Settings struct {
	Uri            string `md:"uri,required"`
	MaxConnections string `md:"maxconnections"`
}

type Input struct {
	WSconnection interface{} `md:"wsconnection,required"`
}

func (o *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"wsconnection": o.WSconnection,
	}
}

func (o *Input) FromMap(values map[string]interface{}) error {

	o.WSconnection = values["wsconnection"]
	return nil
}

type Output struct {
}
