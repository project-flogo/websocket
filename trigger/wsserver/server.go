package wsserver

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/support/log"
)

// Graceful shutdown HttpServer from: https://github.com/corneldamian/httpway/blob/master/server.go

// NewServer create a new server instance
//param server - is a instance of http.Server, can be nil and a default one will be created
func NewServer(addr string, handler http.Handler, enableTLS bool, serverCert string, serverKey string, enableClientAuth bool, trustStore string, tlogger log.Logger) *Server {
	srv := &Server{}
	srv.Server = &http.Server{Addr: addr, Handler: handler}
	srv.enableTLS = enableTLS
	srv.serverCert = serverCert
	srv.serverKey = serverKey
	srv.enableClientAuth = enableClientAuth
	srv.trustStore = trustStore
	srv.logger = tlogger
	return srv
}

//Server the server  structure
type Server struct {
	*http.Server

	serverInstanceID string
	listener         net.Listener
	lastError        error
	serverGroup      *sync.WaitGroup
	clientsGroup     chan bool
	enableTLS        bool
	serverCert       string
	serverKey        string
	enableClientAuth bool
	trustStore       string
	logger           log.Logger
}

// InstanceID the server instance id
func (s *Server) InstanceID() string {
	return s.serverInstanceID
}

// Start this will start server
// command isn't blocking, will exit after run
func (s *Server) Start() error {
	if s.Handler == nil {
		return errors.New("No server handler set")
	}

	if s.listener != nil {
		return errors.New("Server already started")
	}

	addr := s.Addr
	if addr == "" {
		addr = ":http"
	}

	hostname, _ := os.Hostname()
	s.serverInstanceID = fmt.Sprintf("%x", md5.Sum([]byte(hostname+addr)))

	if s.enableTLS {
		//TLS is enabled, load server certificate & key files
		s.logger.Info("Reading certificates")
		var cer tls.Certificate
		if strings.HasPrefix(s.serverKey, "{") || strings.Contains(s.serverKey, ",") || strings.HasPrefix(s.serverKey, "-----") {
			// certfile uploaded in FE via filepicker, or keys configured via app property in base64 encoded, or raw form
			privateKey, err := decodeCerts(s.serverKey, s.logger)
			if err != nil {
				return err
			}
			caCertificate, err := decodeCerts(s.serverCert, s.logger)
			if err != nil {
				return err
			}
			cer, err = tls.X509KeyPair(caCertificate, privateKey)
			if err != nil {
				fmt.Printf("Error while loading certificates - %v", err)
				return err
			}
		} else {
			// configured cert file path in OSS way
			var err error
			cer, err = tls.LoadX509KeyPair(s.serverCert, s.serverKey)
			if err != nil {
				fmt.Printf("Error while loading certificates - %v", err)
				return err
			}
		}
		var config *tls.Config
		if s.enableClientAuth {
			caCertPool, err := getCerts(s.trustStore)
			if err != nil {
				fmt.Printf("Error while loading client trust store - %v", err)
				return err
			}

			config = &tls.Config{Certificates: []tls.Certificate{cer},
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  caCertPool}
			config.BuildNameToCertificate()
		} else {
			config = &tls.Config{Certificates: []tls.Certificate{cer}}
		}

		// bind secure listener
		listener, err := tls.Listen("tcp", addr, config)
		if err != nil {
			return err
		}
		s.listener = listener
	} else {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		s.listener = listener
	}

	s.serverGroup = &sync.WaitGroup{}
	s.clientsGroup = make(chan bool, 50000)

	s.Handler = &serverHandler{s.Handler, s.clientsGroup, s.serverInstanceID}

	s.serverGroup.Add(1)
	go func() {
		defer s.serverGroup.Done()
		err := s.Serve(s.listener)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			s.lastError = err
		}
	}()

	return nil
}

// Stop sends stop command to the server
func (s *Server) Stop() error {
	if s.listener == nil {
		return errors.New("Server not started")
	}

	if err := s.listener.Close(); err != nil {
		return err
	}

	return s.lastError
}

// IsStarted checks if the server is started
// will return true even if the server is stopped but there are still some requests to finish
func (s *Server) IsStarted() bool {
	if s.listener != nil {
		return true
	}

	if len(s.clientsGroup) > 0 {
		return true
	}

	return false
}

// WaitStop waits until server is stopped and all requests are finish
// timeout - is the time to wait for the requests to finish after the server is stopped
// will return error if there are still some requests not finished
func (s *Server) WaitStop(timeout time.Duration) error {
	if s.listener == nil {
		return errors.New("Server not started")
	}

	s.serverGroup.Wait()

	checkClients := time.Tick(100 * time.Millisecond)
	timeoutTime := time.NewTimer(timeout)

	for {
		select {
		case <-checkClients:
			if len(s.clientsGroup) == 0 {
				return s.lastError
			}
		case <-timeoutTime.C:
			return fmt.Errorf("WaitStop error, timeout after %s waiting for %d client(s) to finish", timeout, len(s.clientsGroup))
		}
	}
}

type serverHandler struct {
	handler          http.Handler
	clientsGroup     chan bool
	serverInstanceID string
}

func (sh *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sh.clientsGroup <- true
	defer func() {
		<-sh.clientsGroup
	}()

	w.Header().Add("X-Server-Instance-Id", sh.serverInstanceID)

	sh.handler.ServeHTTP(w, r)
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

func decodeCerts(certVal string, tlogger log.Logger) ([]byte, error) {
	if certVal == "" {
		return nil, fmt.Errorf("Certificate is Empty")
	}

	//if certificate comes from fileselctor it will be base64 encoded
	if strings.HasPrefix(certVal, "{") {
		tlogger.Info("Certificate received from FileSelector")
		certObj, err := coerce.ToObject(certVal)
		if err == nil {
			certRealValue, ok := certObj["content"].(string)
			tlogger.Info("Fetched Content from Certificate Object")
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
		tlogger.Debug("Certificate received from App properties with encoding")
		encoding := certVal[:index]
		certRealValue := certVal[index+1:]

		if strings.EqualFold(encoding, "base64") {
			return base64.StdEncoding.DecodeString(certRealValue)
		}
		return nil, fmt.Errorf("Error in parsing the certificates Or we may be not be supporting the given encoding")
	}

	tlogger.Debug("Certificate received from App properties without encoding")

	//===========These blocks of code to be removed after sriharsha fixes FLOGO-2673=================================
	first := strings.TrimSpace(certVal[:strings.Index(certVal, "----- ")] + "-----")
	middle := strings.TrimSpace(certVal[strings.Index(certVal, "----- ")+5 : strings.Index(certVal, " -----")])
	strings.Replace(middle, " ", "\n", -1)
	last := strings.TrimSpace(certVal[strings.Index(certVal, " -----"):])
	certVal = first + "\n" + middle + "\n" + last
	//===========These blocks of code to be removed after sriharsha fixes FLOGO-2673=================================

	return []byte(certVal), nil
}
