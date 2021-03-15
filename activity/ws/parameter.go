package ws

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/project-flogo/core/activity"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/schema"
	"github.com/project-flogo/core/support/log"
)

/**

Support type for query and path param
    "headers":
        {
            "Content_type": "Content-Type",
            "UserName": "Tracy Li"
        }
    ,
    "path_params":
        {
            "petID": 1101
        }
    ],
    "query_params":
        {
            "count": 10
        }
*/

type Parameters struct {
	Headers        []*TypedValue `json:"headers"`
	PathParams     []*TypedValue `json:"pathParams"`
	QueryParams    []*TypedValue `json:"queryParams"`
	RequestType    string
	ResponseType   string
	ResponseOutput string
}

func (p *Parameters) String(log log.Logger) string {
	v, err := json.Marshal(p)
	if err != nil {
		log.Errorf("Parameter object to string err %s", err.Error())
		return ""
	}
	return string(v)
}

type TypedValue struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

type Param struct {
	Name      string
	Type      string
	Repeating string
	Required  string
}

type ResponseCode struct {
	Code         int64
	responseBody string
}

func ParseParams(paramSchema map[string]interface{}) ([]Param, error) {

	if paramSchema == nil {
		return nil, nil
	}

	var parameter []Param

	//Structure expected to be JSON schema like
	props := paramSchema["properties"].(map[string]interface{})
	for k, v := range props {
		param := &Param{}
		param.Name = k
		propValue := v.(map[string]interface{})
		for k1, v1 := range propValue {
			if k1 == "required" {
				param.Required = v1.(string)
			} else if k1 == "type" {
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

func (t *TypedValue) ToString(log log.Logger) string {
	if t.Value != nil {
		v, err := coerce.ToString(t.Value)
		if err != nil {
			log.Error("Typed value %+v to string error %s", t, err.Error())
			return ""
		}
		return v
	}
	return ""
}

func GetParameter(context activity.Context,input *Input, log log.Logger) (params *Parameters, err error) {
	params = &Parameters{}
	//Headers
	log.Debug("Reading Request Header parameters")
	headersMap, err := LoadJsonSchemaFromInput(context, "headers")
	if err != nil {
		return nil, fmt.Errorf("error loading headers input schema: %s", err.Error())
	}

	if headersMap != nil {
		headers, err := ParseParams(headersMap)
		if err != nil {
			return params, err
		}

		if headers != nil {
			inputHeaders := input.Headers //context.GetInput("headers").(map[string]interface{})
			var typeValuesHeaders []*TypedValue
			for _, hParam := range headers {
				isRequired := hParam.Required
				paramName := hParam.Name
				if isRequired == "true" && inputHeaders[paramName] == nil {
					return nil, fmt.Errorf("Required header parameter [%s] is not configured.", paramName)
				}
				if inputHeaders[paramName] != nil {
					if hParam.Repeating == "true" {
						val := inputHeaders[paramName]
						switch reflect.TypeOf(val).Kind() {
						case reflect.Slice:
							s := reflect.ValueOf(val)

							typeValue := &TypedValue{}
							typeValue.Name = paramName
							typeValue.Type = hParam.Type

							var multiVs []string
							for i := 0; i < s.Len(); i++ {
								sv, _ := coerce.ToString(s.Index(i).Interface())

								multiVs = append(multiVs, sv)
							}

							typeValue.Value = strings.Join(multiVs, ",")
							typeValuesHeaders = append(typeValuesHeaders, typeValue)
						}
					} else {
						typeValue := &TypedValue{}
						typeValue.Name = paramName
						typeValue.Value = inputHeaders[paramName]
						typeValue.Type = hParam.Type
						typeValuesHeaders = append(typeValuesHeaders, typeValue)
					}
					params.Headers = typeValuesHeaders
				}
			}
		}
	}
	//Query Parameters
	log.Debug("Reading Query parameters")
	queryParamsMap, err := LoadJsonSchemaFromInput(context, "queryParams")
	if err != nil {
		return nil, fmt.Errorf("error loading queryParams input schema: %s", err.Error())
	}
	if queryParamsMap != nil {
		queryParams, err := ParseParams(queryParamsMap)
		if err != nil {
			return params, err
		}

		if queryParams != nil {
			inputQueries := input.QueryParams //context.GetInput("queryParams").(map[string]interface{})
			var typeValuesQueries []*TypedValue
			for _, qParam := range queryParams {
				isRequired := qParam.Required
				paramName := qParam.Name
				if isRequired == "true" && inputQueries[paramName] == nil {
					return nil, fmt.Errorf("Required query parameter [%s] is not configured.", paramName)
				}

				if inputQueries[paramName] != nil {
					if qParam.Repeating == "true" {
						val := inputQueries[paramName]
						switch reflect.TypeOf(val).Kind() {
						case reflect.Slice:
							s := reflect.ValueOf(val)
							for i := 0; i < s.Len(); i++ {
								typeValue := &TypedValue{}
								typeValue.Name = paramName
								typeValue.Value = s.Index(i).Interface()
								typeValue.Type = qParam.Type
								typeValuesQueries = append(typeValuesQueries, typeValue)
							}
						}
					} else {
						typeValue := &TypedValue{}
						typeValue.Name = paramName
						typeValue.Value = inputQueries[paramName]
						typeValue.Type = qParam.Type
						typeValuesQueries = append(typeValuesQueries, typeValue)
					}
					params.QueryParams = typeValuesQueries
				}
			}
		}

	}

	//Path parameters
	log.Debug("Reading Path parameters")
	pathParamsMap, err := LoadJsonSchemaFromInput(context, "pathParams")
	if err != nil {
		return nil, fmt.Errorf("error loading pathParams input schema: %s", err.Error())
	}
	if pathParamsMap != nil {
		pathParams, err := ParseParams(pathParamsMap)
		if err != nil {
			return params, err
		}
		if pathParams != nil {
			inputPathParams := input.PathParams //context.GetInput("pathParams").(map[string]interface{}) //
			var typeValuesPath []*TypedValue
			for _, pParam := range pathParams {
				paramName := pParam.Name
				if inputPathParams[paramName] == "" {
					return nil, fmt.Errorf("Required path parameter [%s] is not configured.", paramName)
				}

				typeValue := &TypedValue{}
				typeValue.Name = paramName
				typeValue.Value = inputPathParams[paramName]
				typeValue.Type = pParam.Type
				typeValuesPath = append(typeValuesPath, typeValue)
				params.PathParams = typeValuesPath

			}
		}
	}

	/*requestType := input.RequestType
	if requestType == "" {
		requestType = "application/json"
	}

	responseType, _ := coerce.ToString(context.(*instance.TaskInst).Task().ActivityConfig().GetOutput("responseType"))
	if responseType == "" {
		responseType = "application/json"
	}

	responseOutput, _ := coerce.ToString(context.(*instance.TaskInst).Task().ActivityConfig().GetOutput("responseOutput"))
	if responseOutput == "" {
		responseOutput = "application/json"
	}

	//params.RequestType = requestType
	params.ResponseType = responseType
	params.ResponseOutput = responseOutput*/

	return params, err
}

func LoadJsonSchemaFromInput(context activity.Context, attributeName string) (map[string]interface{}, error) {
	var schemaModel schema.Schema
	if sIO, ok := context.(schema.HasSchemaIO); ok {
		schemaModel = sIO.GetInputSchema(attributeName)
		if schemaModel != nil {
			return coerce.ToObject(schemaModel.Value())
		}
	}
	return nil, nil
}

func LoadJsonSchemaFromOutput(context activity.Context, attributeName string) (map[string]interface{}, error) {
	var schemaModel schema.Schema
	if sIO, ok := context.(schema.HasSchemaIO); ok {
		schemaModel = sIO.GetOutputSchema(attributeName)
		if schemaModel != nil {
			return coerce.ToObject(schemaModel.Value())
		}
	}
	return nil, nil
}

func ParseParameters(parameters interface{}) (*Parameters, error) {
	parameter := &Parameters{}

	switch parameters.(type) {
	case string:
		err := json.Unmarshal([]byte(parameters.(string)), parameter)
		if err != nil {
			return nil, err
		}
	default:
		b, err := json.Marshal(parameters)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(b, parameter)
		if err != nil {
			return nil, err
		}

	}
	return parameter, nil
}
