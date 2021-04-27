package ws

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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
		settings:      s,
		cachedClients: sync.Map{},
		continuePing:  true,
	}
	return act, nil
}

// Activity is an activity that is used to invoke a Web socket operation
type Activity struct {
	settings      *Settings
	cachedClients sync.Map
	continuePing  bool
	actLogger     log.Logger
}

// Metadata returns the metadata for a websocket client
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Invokes a web socket operation
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {
	a.actLogger = ctx.Logger()
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
	if !ok {
		var dialer websocket.Dialer
		if isWSS {
			tlsconfig := &tls.Config{}
			allowInsecure := a.settings.AllowInsecure
			if allowInsecure {
				tlsconfig.InsecureSkipVerify = true
			} else {
				var cacertObj map[string]interface{}
				if a.settings.CaCert != "" {
					err = json.Unmarshal([]byte(a.settings.CaCert), &cacertObj)
					if err != nil { //file path configured
						certPool, err := getCerts(a.settings.CaCert)
						if err != nil {
							ctx.Logger().Errorf("Error while loading client trust store - %v", err)
							return false, err
						}
						tlsconfig.RootCAs = certPool
					} else { // file content configured
						rootCAbytes, err := decodeCerts(a.settings.CaCert, ctx.Logger())
						if err != nil {
							ctx.Logger().Errorf("Error while loading client trust store content - %v", err)
							return false, err
						}
						certPool := x509.NewCertPool()
						certsAdded := certPool.AppendCertsFromPEM(rootCAbytes)
						if !certsAdded {
							ctx.Logger().Error("Unsupported certificate found. It must be a valid PEM certificate.")
							return false, activity.NewError("Unsupported certificate found. It must be a valid PEM certificate.", "", nil)
						}
						tlsconfig.RootCAs = certPool
					}
				}
			}
			dialer = websocket.Dialer{TLSClientConfig: tlsconfig}
		} else {
			dialer = *websocket.DefaultDialer
		}
		ctx.Logger().Debug("Creating new connection")
		ctx.Logger().Infof("dialing websocket endpoint[%s]...", builtURL)
		ctx.Logger().Debugf("dialing websocket endpoint with headers[%s]...", h)
		conn, res, err := dialer.Dial(builtURL, h)
		if err != nil {
			if res != nil {
				defer res.Body.Close()
				body, err1 := ioutil.ReadAll(res.Body)
				if err1 != nil {
					ctx.Logger().Errorf("response code is: %v , error while reading response payload is: %s ", res.StatusCode, err1)
				}
				ctx.Logger().Errorf("response code is: %v , payload is: %s , error is: %s", res.StatusCode, string(body), err)
			}
			return false, err
		}
		a.cachedClients.Store(key, conn)
		connection = conn

		// send ping to avoid connection timeout, for newly created connection only as its goroutine
		connection.SetPongHandler(func(msg string) error { /* ws.SetReadDeadline(time.Now().Add(pongWait)); */
			ctx.Logger().Debugf("received pong msg from server: %s", msg)
			return nil
		})
		// send ping to avoid TCI connection timeout
		go ping(connection, a)
	} else {
		ctx.Logger().Debug("Reusing connection from cache")
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
			ctx.Logger().Debug("Deleting connection from cache due to error")
			a.cachedClients.Delete(key)
			return false, err
		}
	} else {
		return false, errors.New("Message is not configured")
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
			vSlice := strings.Split(v.ToString(ctx.Logger()), ",")
			header[v.Name] = vSlice
		}
	}
	return header
}

func decodeCerts(certVal string, log log.Logger) ([]byte, error) {
	if certVal == "" {
		return nil, fmt.Errorf("Certificate is Empty")
	}

	//if certificate comes from fileselctor it will be base64 encoded
	if strings.HasPrefix(certVal, "{") {
		log.Info("Certificate received from FileSelector")
		certObj, err := coerce.ToObject(certVal)
		if err == nil {
			certRealValue, ok := certObj["content"].(string)
			log.Infof("Fetched Content from Certificate Object")
			if !ok || certRealValue == "" {
				return nil, fmt.Errorf("Didn't found the certificate content")
			}

			index := strings.IndexAny(certRealValue, ",")
			if index > -1 {
				certRealValue = certRealValue[index+1:]
			}

			return base64.StdEncoding.DecodeString(certRealValue)
		}
		return nil, err
	}

	//if the certificate comes from application properties need to check whether that it contains , ans encoding
	index := strings.IndexAny(certVal, ",")

	if index > -1 {
		//some encoding is there
		log.Debugf("Certificate received from App properties with encoding")
		encoding := certVal[:index]
		certRealValue := certVal[index+1:]

		if strings.EqualFold(encoding, "base64") {
			return base64.StdEncoding.DecodeString(certRealValue)
		}
		return nil, fmt.Errorf("Error in parsing the certificates Or we may be not be supporting the given encoding")
	}

	log.Debugf("Certificate received from App properties without encoding")

	first := strings.TrimSpace(certVal[:strings.Index(certVal, "----- ")] + "-----")
	middle := strings.TrimSpace(certVal[strings.Index(certVal, "----- ")+5 : strings.Index(certVal, " -----")])
	strings.Replace(middle, " ", "\n", -1)
	last := strings.TrimSpace(certVal[strings.Index(certVal, " -----"):])
	certVal = first + "\n" + middle + "\n" + last

	return []byte(certVal), nil
}

func getCerts(trustStore string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	fileInfo, err := os.Stat(trustStore)
	if err != nil {
		return certPool, fmt.Errorf("Truststore [%s] does not exist", trustStore)
	}
	switch mode := fileInfo.Mode(); {
	case mode.IsDir():
		break
	case mode.IsRegular():
		return certPool, fmt.Errorf("Truststore [%s] is not a directory.  Must be a directory containing trusted certificates in PEM format",
			trustStore)
	}
	trustedCertFiles, err := ioutil.ReadDir(trustStore)
	if err != nil || len(trustedCertFiles) == 0 {
		return certPool, fmt.Errorf("Failed to read trusted certificates from [%s]  Must be a directory containing trusted certificates in PEM format", trustStore)
	}
	for _, trustCertFile := range trustedCertFiles {
		fqfName := fmt.Sprintf("%s%c%s", trustStore, os.PathSeparator, trustCertFile.Name())
		trustCertBytes, err := ioutil.ReadFile(fqfName)
		if err != nil {
			fmt.Errorf("Failed to read trusted certificate [%s] ... continueing", trustCertFile.Name())
		}
		certPool.AppendCertsFromPEM(trustCertBytes)
	}
	if len(certPool.Subjects()) < 1 {
		return certPool, fmt.Errorf("Failed to read trusted certificates from [%s]  After processing all files in the directory no valid trusted certs were found", trustStore)
	}
	return certPool, nil
}

func ping(connection *websocket.Conn, a *Activity) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if a.continuePing {
			select {
			case t := <-ticker.C:
				a.actLogger.Debugf("Sending Ping at timestamp : %v", t)
				if err := connection.WriteControl(websocket.PingMessage, []byte("---HeartBeat---"), time.Now().Add(time.Second)); err != nil {
					a.actLogger.Errorf("error while sending ping: %v", err)
					var ErrCloseSent = errors.New("websocket: close sent")
					if err != ErrCloseSent {
						e, ok := err.(net.Error)
						if !ok || !e.Temporary() {
							a.actLogger.Warnf("stopping ping ticker for conn: %v as received non temporary error while sending ping: %s ", connection.UnderlyingConn(), err.Error())
							if a.continuePing { // remove connection from cache only if engine is not in shutting down state
								a.cachedClients.Range(func(key, value interface{}) bool {
									conn, ok := value.(*websocket.Conn)
									if ok && (connection == conn) {
										a.actLogger.Warnf("Removing broken connection from cache: [%v] for key: [%s]", conn.UnderlyingConn(), key)
										a.cachedClients.Delete(key)
										return false
									}
									return true
								})
							}
							return
						}
					}
				}
			}
		} else {
			a.actLogger.Debugf("stopping ping ticker for conn: %v while engine getting stopped", connection.UnderlyingConn())
			return
		}
	}
}

func (a *Activity) Cleanup() error {
	a.continuePing = false
	a.cachedClients.Range(func(key, value interface{}) bool {
		conn, ok := value.(*websocket.Conn)
		if !ok {
			a.actLogger.Info("value is not websocket connection type to close while activity cleanup")
		} else {
			err := conn.WriteControl(websocket.CloseMessage, []byte("Close connection while Activity cleanup"), time.Now().Add(time.Second))
			if err != nil {
				a.actLogger.Warnf("error while sending close message: %v", err)
			}
			err1 := conn.Close()
			if err1 != nil {
				a.actLogger.Infof("error while closing connection: %v", err1)
			}
			a.actLogger.Infof("Connection closed while activity cleanup.....")
		}
		return true
	})
	return nil
}
