package wsclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/project-flogo/core/data/metadata"

	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/action"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/support/log"
	"github.com/project-flogo/core/trigger"
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
	runner       action.Runner
	wsconn       *websocket.Conn
	settings     *Settings
	logger       log.Logger
	config       *trigger.Config
	tInitContext trigger.InitContext
	continuePing bool
}

// New implements trigger.Factory.New
func (*Factory) New(config *trigger.Config) (trigger.Trigger, error) {
	s := &Settings{}
	err := metadata.MapToStruct(config.Settings, s, true)
	if err != nil {
		return nil, err
	}
	return &Trigger{settings: s, config: config, continuePing: true}, nil
}

// Initialize initializes the trigger
func (t *Trigger) Initialize(ctx trigger.InitContext) error {
	t.logger = ctx.Logger()
	headers := t.settings.Headers
	queryParams := t.settings.QueryParams
	urlstring := t.settings.URL
	// populate headers
	header := make(http.Header)
	if len(headers) > 0 {
		for key, val := range headers {
			splittedSlice := strings.Split(val, ",")
			var hvalues []string
			for _, val := range splittedSlice {
				if len(strings.TrimSpace(val)) > 0 {
					hvalues = append(hvalues, val)
				}
			}
			header[key] = hvalues
		}
	}
	// populate queryparam
	if len(queryParams) > 0 {
		qp := url.Values{}
		for key, val := range queryParams {
			splittedSlice := strings.Split(val, ",")
			for _, splittedval := range splittedSlice {
				if len(strings.TrimSpace(splittedval)) > 0 {
					qp.Add(key, splittedval)
				}
			}
		}
		urlstring = urlstring + "?" + qp.Encode()
	}
	var isWSS bool
	if urlstring != "" {
		isWSS = strings.HasPrefix(urlstring, "wss")
	}
	var dialer websocket.Dialer
	if isWSS {
		tlsconfig := &tls.Config{}
		allowInsecure := t.settings.AllowInsecure
		if allowInsecure {
			tlsconfig.InsecureSkipVerify = true
		} else {
			caCertValue := t.settings.CaCert
			var cacertObj map[string]interface{}
			if caCertValue != "" {
				err := json.Unmarshal([]byte(caCertValue), &cacertObj)
				if err != nil { // filepath configured
					certPool, err := getCerts(caCertValue)
					if err != nil {
						t.logger.Errorf("Error while loading client trust store - %v", err)
						return err
					}
					tlsconfig.RootCAs = certPool
				} else { // file content configured
					rootCAbytes, err := decodeCerts(caCertValue, t.logger)
					if err != nil {
						t.logger.Errorf("Error while loading client trust store content - %v", err)
						return err
					}
					certPool := x509.NewCertPool()
					certsAdded := certPool.AppendCertsFromPEM(rootCAbytes)
					if !certsAdded {
						t.logger.Error("Unsupported certificate found. It must be a valid PEM certificate.")
						return fmt.Errorf("Unsupported certificate found. It must be a valid PEM certificate.")
					}
					tlsconfig.RootCAs = certPool
				}
			}
		}
		dialer = websocket.Dialer{TLSClientConfig: tlsconfig}
	} else {
		dialer = *websocket.DefaultDialer
	}
	t.logger.Infof("[ %s ] dialing websocket endpoint [%s]...", t.config.Id, urlstring)
	t.logger.Debugf("[ %s ] dialing websocket endpoint with headers [%s]...", t.config.Id, header)

	conn, res, err := dialer.Dial(urlstring, header)
	if err != nil {
		if res != nil {
			defer res.Body.Close()
			body, err1 := ioutil.ReadAll(res.Body)
			if err1 != nil {
				ctx.Logger().Errorf("response code is: %v , error while reading response payload is: %s ", res.StatusCode, err1)
			}
			t.logger.Errorf("response code is: %v , payload is: %s , error is: %s", res.StatusCode, string(body), err)
		}
		return fmt.Errorf("error while connecting to websocket endpoint[%s] - %s", urlstring, err)
	}

	t.wsconn = conn
	// set ponghanlder to print the received pong message from server
	conn.SetPongHandler(func(msg string) error { /* ws.SetReadDeadline(time.Now().Add(pongWait)); */
		ctx.Logger().Debugf("received pong msg from server: %s", msg)
		return nil
	})
	// send ping to avoid TCI connection timeout
	go ping(conn, t)
	t.tInitContext = ctx

	return nil
}

func isJSON(str []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(str, &js) == nil
}

// Start starts the trigger
func (t *Trigger) Start() error {
	if t.wsconn != nil {
		go func() {
			defer func() {
				err := t.wsconn.WriteControl(websocket.CloseMessage, []byte("Sending close message while getting out of reading connection loop"), time.Now().Add(time.Second))
				if err != nil {
					t.logger.Warnf("Received error [%s] while writing close message", err)
				}
				t.logger.Info("Closing connection while going out of trigger handler")
				t.wsconn.Close()
			}()
			for {
				_, message, err := t.wsconn.ReadMessage()
				if err != nil {
					t.logger.Errorf("error while reading websocket message: %s", err)
					break
				}
				t.logger.Debug("New message received...")
				out := &Output{}
				var content interface{}
				if (t.config.Settings["format"] != nil && t.config.Settings["format"].(string) == "JSON") ||
					(t.config.Settings["format"] == nil && isJSON(message)) {
					err := json.NewDecoder(bytes.NewBuffer(message)).Decode(&content)
					if err != nil {
						t.logger.Errorf("error while decoding websocket message of JSON type : %s", err)
						break
					}
				} else {
					content = string(message)
				}

				out.Content = content
				out.WSconnection = t.wsconn

				for _, handler := range t.tInitContext.GetHandlers() {
					_, err1 := handler.Handle(context.Background(), out)
					if err1 != nil {
						t.logger.Errorf("Run action  failed [%s] ", err1)
					}
				}
			}
			t.logger.Infof("stopped listening to websocket endpoint")
		}()
	} else {
		t.logger.Error("Websocket Connection not initialized")
		return errors.New("Websocket Connection not initialized")
	}
	return nil
}

// Stop stops the trigger
func (t *Trigger) Stop() error {
	t.logger.Infof("Stopping Trigger %s", t.config.Id)
	t.continuePing = false
	err := t.wsconn.WriteControl(websocket.CloseMessage, []byte("Closing connection while stopping trigger"), time.Now().Add(time.Second))
	if err != nil {
		t.logger.Warnf("Error received: [%s] while sending close message when Stopping Trigger", err)
	}
	t.wsconn.Close()
	defer t.logger.Info("Trigger %s Stopped", t.config.Id)
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
				return nil, fmt.Errorf("Did not found the certificate content")
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

func ping(connection *websocket.Conn, tr *Trigger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if tr.continuePing {
			select {
			case t := <-ticker.C:
				tr.logger.Debugf("Sending Ping at timestamp : %v", t)
				if err := connection.WriteControl(websocket.PingMessage, []byte("---HeartBeat---"), time.Now().Add(time.Second)); err != nil {
					tr.logger.Errorf("error while sending ping: %v", err)
					var ErrCloseSent = errors.New("websocket: close sent")
					if err != ErrCloseSent {
						e, ok := err.(net.Error)
						if !ok || !e.Temporary() {
							tr.logger.Debugf("stopping ping ticker for conn: %p as received non temporary error while sending ping: %s ", connection, err.Error())
							return
						}
					}
				}
			}
		} else {
			tr.logger.Debugf("stopping ping ticker for conn: %p while engine getting stopped", connection)
			return
		}
	}
}
