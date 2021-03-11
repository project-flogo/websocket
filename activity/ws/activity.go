package ws

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	//"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/activity"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/metadata"
	"github.com/project-flogo/core/support/log"
)

func init() {
	activity.Register(&Activity{}, New)
}

var activityMd = activity.ToMetadata(&Settings{}, &Input{}, &Output{})

// New create a new websocket client
func New(ctx activity.InitContext) (activity.Activity, error) {
	s := &Settings{}
	act := &Activity{}
	if ctx.Settings()["format"] == nil {
		err := metadata.MapToStruct(ctx.Settings(), s, true)
		if err != nil {
			return nil, err
		}

		connection, _, err := websocket.DefaultDialer.Dial(s.URI, nil)
		if err != nil {
			return nil, err
		}

		act = &Activity{
			settings:   s,
			connection: connection,
			ossVersion: true,
		}

	} else {
		act = &Activity{
			initsettings:  ctx.Settings(),
			cachedClients: sync.Map{},
			ossVersion:    false,
		}
	}
	return act, nil
}

// Activity is an activity that is used to invoke a Web socket operation
type Activity struct {
	settings      *Settings
	connection    *websocket.Conn
	initsettings  map[string]interface{}
	cachedClients sync.Map
	ossVersion    bool
}

// Metadata returns the metadata for a websocket client
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Invokes a web socket operation
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {
	if a.ossVersion {
		input := &Input{}
		ctx.GetInputObject(input)

		var message []byte
		if input.Message != nil {
			if value, ok := input.Message.(string); ok {
				message = []byte(value)
			} else {
				value, err := json.Marshal(input.Message)
				if err != nil {
					return false, err
				}
				message = value
			}
		}

		err = a.connection.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	fmt.Println("******************In FE version")

	fmt.Println("******************ctx", ctx)
	// create connection
	//var uri string
	var isWSS bool
	url := a.initsettings["uri"]
	var urlstr string
	if url != "" {
		urlstr = url.(string) //fmt.Sprintf("%v", url)
		isWSS = strings.HasPrefix(urlstr, "wss")
	}

	var dialer websocket.Dialer
	if isWSS {
		allowInsecure, err := coerce.ToBool(a.initsettings["allowInsecure"])
		if err != nil { //TODO
		}
		if allowInsecure {
			config := &tls.Config{InsecureSkipVerify: true}
			dialer = websocket.Dialer{TLSClientConfig: config}
		}
	} else {
		dialer = *websocket.DefaultDialer
	}
	parameters, err := GetParameter(ctx, ctx.Logger())
	if err != nil {
		ctx.Logger().Error(err)
		return false, err
	}
	fmt.Println("****************params", parameters)
	//populate custom headers
	h := getHeaders(ctx, parameters)
	//populate url with path and query params
	builtURL := buildURI(urlstr, parameters, ctx.Logger())
	fmt.Println("***************builtURL", builtURL)
	connection, res, err := dialer.Dial(builtURL, h)
	if err != nil {
		if res != nil {
			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)
			fmt.Println("body is :", string(body), " err is: ", err)
			ctx.Logger().Infof("res is  %s code is %v , err is %s", res.StatusCode, string(body), err)
		}
		return false, err
	}

	//populate msg
	var message []byte
	if ctx.GetInput("message") != nil {
		if ctx.GetInput("format") != nil && ctx.GetInput("format") == "string" {
			value, err := coerce.ToString(ctx.GetInput("message"))
			if err != nil {
				//TODO
			}
			message = []byte(value)
		} else {
			value, err := json.Marshal(ctx.GetInput("message"))
			if err != nil {
				return false, err
			}
			message = value
		}
	}
	fmt.Println(message)
	//write to connection
	err = connection.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		return false, err
	}

	return true, nil
}

func buildURI(uri string, param *Parameters, log log.Logger) string {
	if param != nil {
		if param.PathParams != nil && len(param.PathParams) > 0 {
			uri = BuildURI(uri, param.PathParams)
		}

		if param.QueryParams != nil && len(param.QueryParams) > 0 {
			qp := url.Values{}
			for _, value := range param.QueryParams {
				qp.Add(value.Name, value.ToString(log))
			}
			uri = uri + "?" + qp.Encode()
		}

	}
	return uri
}

func BuildURI(uri string, values []*TypedValue) string {
	for _, pp := range values {
		data, _ := coerce.ToString(pp.Value)
		uri = strings.Replace(uri, "{"+pp.Name+"}", data, -1)
	}
	return uri
}

func getHeaders(ctx activity.Context, param *Parameters) http.Header {
	header := make(http.Header)
	if param != nil && param.Headers != nil && len(param.Headers) > 0 {
		for _, v := range param.Headers {
			//Any input should oeverride exist header
			// To avoid canonicalization of header name, adding headers directly to the request header map instead of using Add/Set.
			header[v.Name] = []string{v.ToString(ctx.Logger())}
		}
	}
	return header
}
