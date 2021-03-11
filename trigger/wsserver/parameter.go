package wsserver

import (
	"strings"

	"github.com/project-flogo/core/data/coerce"
)

type Parameter struct {
	Name      string `json:"parameterName"`
	Type      string `json:"type"`
	Repeating string `json:"repeating,omitempty"`
	Required  string `json:"required,omitempty"`
}

func ParseParams(paramSchema map[string]interface{}) ([]Parameter, error) {

	if paramSchema == nil {
		return nil, nil
	}

	var parameter []Parameter

	//Structure expected to be JSON schema like
	props := paramSchema["properties"].(map[string]interface{})
	for k, v := range props {
		param := &Parameter{}
		param.Name = k

		//if the k is required or not
		requiredKeys, ok := paramSchema["required"].([]interface{})
		if ok {
			for ran := range requiredKeys {
				if strings.EqualFold(k, requiredKeys[ran].(string)) {
					param.Required = "true"
					break
				}

			}
		}

		propValue := v.(map[string]interface{})
		for k1, v1 := range propValue {
			if k1 == "type" {
				if v1 != "array" {
					param.Repeating = "false"
				}
				param.Type = v1.(string)
			} else if k1 == "items" {
				param.Repeating = "true"
				items := v1.(map[string]interface{})
				s, err := coerce.ToString(items["type"])
				if err != nil {
					return nil, err
				}
				param.Type = s
			}
		}
		parameter = append(parameter, *param)
	}

	return parameter, nil
}
