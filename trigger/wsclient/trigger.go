package wsclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/action"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/support/log"
	"github.com/project-flogo/core/trigger"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var triggerMd = trigger.NewMetadata(&Settings{}, &Output{})

func init() {
	trigger.Register(&Trigger{}, &Factory{})
}

// Factory for creating triggers
type Factory struct {
}

// Metadata implements trigger.Factory.Metadata
func (*Factory) Metadata() *trigger.Metadata {
	return triggerMd
}

// Trigger trigger struct
type Trigger struct {
	runner   action.Runner
	wsconn   *websocket.Conn
	settings *Settings
	logger   log.Logger
	config   *trigger.Config
}

// New implements trigger.Factory.New
func (*Factory) New(config *trigger.Config) (trigger.Trigger, error) {
	s := &Settings{}
	/*err := metadata.MapToStruct(config.Settings, s, true)
	if err != nil {
		return nil, err
	}*/

	return &Trigger{settings: s, config: config}, nil
}

// Initialize initializes the trigger
func (t *Trigger) Initialize(ctx trigger.InitContext) error {
	t.logger = ctx.Logger()
	urlstring, err := coerce.ToString(t.config.Settings["url"])
	// populate headers
	header := make(http.Header)
	headerObject, err := coerce.ToObject(t.config.Settings["headers"])
	if err != nil {
		// queryparam is array as per FE
		headerArray, err := coerce.ToArray(t.config.Settings["headers"])
		if err != nil {
			return err
		}
		for _, val := range headerArray {
			qparamMap := val.(map[string]interface{})
			header[qparamMap["parameterName"].(string)] = []string{qparamMap["value"].(string)}
		}
	} else {
		fmt.Println(headerObject)
		// OSS way
		//TODO
	}

	// populate queryparam
	qparamObject, err := coerce.ToObject(t.config.Settings["queryParams"])
	if err != nil {
		// queryparam is array as per FE
		qparamArray, err := coerce.ToArray(t.config.Settings["queryParams"])
		if err != nil {
			return err
		}
		qp := url.Values{}
		for _, val := range qparamArray {
			qparamMap := val.(map[string]interface{})
			qp.Add(qparamMap["parameterName"].(string), qparamMap["value"].(string))
		}
		urlstring = urlstring + "?" + qp.Encode()
	} else {
		//OSS way
		fmt.Println(qparamObject)
		//TODO
	}

	var isWSS bool
	if urlstring != "" {
		isWSS = strings.HasPrefix(urlstring, "wss")
	}
	var dialer websocket.Dialer
	if isWSS {
		tlsconfig := &tls.Config{}
		allowInsecure := t.config.Settings["queryParams"].(bool)
		if allowInsecure {
			tlsconfig.InsecureSkipVerify = true
		} else {
			// identify if OSS
			var cacertObj map[string]interface{}
			if t.config.Settings["caCert"].(string) != "" {
				err = json.Unmarshal([]byte(t.config.Settings["caCert"].(string)), &cacertObj)
				if err != nil { //OSS way
					certPool, err := getCerts(a.settings.CaCert)
					if err != nil {
						fmt.Printf("Error while loading client trust store - %v", err)
						return err
					}
					tlsconfig.RootCAs = certPool
				} else { // flogo way
					rootCAbytes, err := decodeCerts(a.settings.CaCert, ctx.Logger())
					if err != nil {
						ctx.Logger().Error(err)
						return err
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

	t.logger.Infof("dialing websocket endpoint[%s]...", urlstring)
	conn, res, err := dialer.Dial(urlstring, header)
	if err != nil {
		if res != nil {
			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)
			ctx.Logger().Infof("res code is %v payload is %s , err is %s", res.StatusCode, string(body), err)
		}
		return fmt.Errorf("error while connecting to websocket endpoint[%s] - %s", urlstring, err)
	}

	t.wsconn = conn
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			t.logger.Infof("Message received :", string(message))
			if err != nil {
				t.logger.Errorf("error while reading websocket message: %s", err)
				break
			}

			for _, handler := range ctx.GetHandlers() {
				out := &Output{}
				out.Content = message
				_, err := handler.Handle(context.Background(), out)
				if err != nil {
					t.logger.Errorf("Run action  failed [%s] ", err)
				}
			}
		}
		t.logger.Infof("stopped listening to websocket endpoint")
	}()
	return nil
}

// Start starts the trigger
func (t *Trigger) Start() error {
	return nil
}

// Stop stops the trigger
func (t *Trigger) Stop() error {
	t.wsconn.Close()
	return nil
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
