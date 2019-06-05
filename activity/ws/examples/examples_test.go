package examples

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/engine"
	"github.com/project-flogo/microgateway/api"
	test "github.com/project-flogo/websocket/internal/testing"
	"github.com/stretchr/testify/assert"
)

var messages = make(chan string, 8)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("upgrade error", err)
	} else {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read error", err)
				break
			}
			messages <- string(message)
		}
		return
	}
}

func testApplication(t *testing.T, e engine.Engine) {
	defer api.ClearResources()

	test.Drain("9096")
	err := e.Start()
	assert.Nil(t, err)
	defer func() {
		assert.Nil(t, e.Stop())
	}()
	test.Pour("9096")

	transport := &http.Transport{
		MaxIdleConns: 1,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
	}
	request := func(payload string) []byte {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:9096/message", bytes.NewReader([]byte(payload)))
		assert.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		assert.Nil(t, err)
		body, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err)
		response.Body.Close()
		return body
	}
	message := `{"message":"hello world"}`
	request(message)
	response := <-messages
	assert.Equal(t, message, response)
}

func TestIntegrationAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}

	test.Drain("8080")
	middleware := http.NewServeMux()
	middleware.HandleFunc("/ws", wsHandler)
	s := &http.Server{
		Addr:    "localhost:8080",
		Handler: middleware,
	}
	go func() {
		s.ListenAndServe()
	}()
	test.Pour("8080")
	defer s.Shutdown(context.Background())

	e, err := Example()
	assert.Nil(t, err)
	testApplication(t, e)
}

func TestIntegrationJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping JSON integration test in short mode")
	}

	test.Drain("8080")
	middleware := http.NewServeMux()
	middleware.HandleFunc("/ws", wsHandler)
	s := &http.Server{
		Addr:    "localhost:8080",
		Handler: middleware,
	}
	go func() {
		s.ListenAndServe()
	}()
	test.Pour("8080")
	defer s.Shutdown(context.Background())

	data, err := ioutil.ReadFile(filepath.FromSlash("./json/flogo.json"))
	assert.Nil(t, err)
	cfg, err := engine.LoadAppConfig(string(data), false)
	assert.Nil(t, err)
	e, err := engine.New(cfg)
	assert.Nil(t, err)
	testApplication(t, e)
}
