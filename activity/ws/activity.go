package ws

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

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
		settings:      s,
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
	if !ok {
		var dialer websocket.Dialer
		if isWSS {
			tlsconfig := &tls.Config{}
			allowInsecure := a.settings.AllowInsecure
			if allowInsecure {
				tlsconfig.InsecureSkipVerify = true
			} else {
				// identify if OSS
				var cacertObj map[string]interface{}
				if a.settings.CaCert != "" {
					err = json.Unmarshal([]byte(a.settings.CaCert), &cacertObj)
					if err != nil { //OSS way
						certPool, err := getCerts(a.settings.CaCert)
						if err != nil {
							fmt.Printf("Error while loading client trust store - %v", err)
							return false, err
						}
						tlsconfig.RootCAs = certPool
					} else { // flogo way
						rootCAbytes, err := decodeCerts(a.settings.CaCert, ctx.Logger())
						if err != nil {
							ctx.Logger().Error(err)
							return false, err
						}
						certPool := x509.NewCertPool()
						certPool.AppendCertsFromPEM(rootCAbytes)
						tlsconfig.RootCAs = certPool
					}
				}
			}
			dialer = websocket.Dialer{TLSClientConfig: tlsconfig}
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
	} else {
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

	//===========These blocks of code to be removed after sriharsha fixes FLOGO-2673=================================
	first := strings.TrimSpace(certVal[:strings.Index(certVal, "----- ")] + "-----")
	middle := strings.TrimSpace(certVal[strings.Index(certVal, "----- ")+5 : strings.Index(certVal, " -----")])
	strings.Replace(middle, " ", "\n", -1)
	last := strings.TrimSpace(certVal[strings.Index(certVal, " -----"):])
	certVal = first + "\n" + middle + "\n" + last
	//===========These blocks of code to be removed after sriharsha fixes FLOGO-2673=================================

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
