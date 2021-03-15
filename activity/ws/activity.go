package ws

import (
	"crypto/tls"
	"fmt"
	"github.com/pkg/errors"
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
	err := metadata.MapToStruct(ctx.Settings(), s, true)
	if err != nil {
		return nil, err
	}
	act := &Activity{
		settings:  s,
		cachedClients: sync.Map{},
	}
	return act, nil
}

// Activity is an activity that is used to invoke a Web socket operation
type Activity struct {
	settings      *Settings
	cachedClients sync.Map
}

// Metadata returns the metadata for a websocket client
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Invokes a web socket operation
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {
	input := &Input{}
	err = ctx.GetInputObject(input)
	if err != nil {
		return false, err
	}
	var isWSS bool
	url := a.settings.URI
	if url != "" {
		isWSS = strings.HasPrefix(url, "wss")
	}
	parameters, err := GetParameter(ctx, input, ctx.Logger())
	if err != nil {
		ctx.Logger().Error(err)
		return false, err
	}
	//populate custom headers
	h := getHeaders(ctx, parameters)
	//populate url with path and query params
	builtURL := buildURI(url, parameters, ctx.Logger())
	key := ctx.ActivityHost().Name() + "-" + ctx.Name() + "-" + builtURL + "-" + fmt.Sprintf("%v", h)
	cachedConnection, ok := a.cachedClients.Load(key)
	var connection *websocket.Conn
	if !ok{
		var dialer websocket.Dialer
		if isWSS {
			allowInsecure := a.settings.AllowInsecure
			if allowInsecure {
				config := &tls.Config{InsecureSkipVerify: true}
				dialer = websocket.Dialer{TLSClientConfig: config}
			} else {
				//TODO
			}
		} else {
			dialer = *websocket.DefaultDialer
		}
		ctx.Logger().Info("Creating new connection")
		conn, res, err := dialer.Dial(builtURL, h)
		if err != nil {
			if res != nil {
				defer res.Body.Close()
				body, err := ioutil.ReadAll(res.Body)
				ctx.Logger().Infof("res code is %v payload is %s , err is %s", res.StatusCode, string(body), err)
			}
			return false, err
		}
		a.cachedClients.Store(key, conn)
		connection = conn
	}else{
		ctx.Logger().Info("Reusing connection from cache")
		connection = cachedConnection.(*websocket.Conn)
	}


	//populate msg
	if input.Message != nil {
		message, err := coerce.ToBytes(input.Message)
		if err != nil {
			return false, err
		}
		err = connection.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return false, err
		}
	} else {
		return false, errors.New("Message is non configured")
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
