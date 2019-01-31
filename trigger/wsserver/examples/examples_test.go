package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/project-flogo/core/engine"
	"github.com/project-flogo/microgateway/api"
	test "github.com/project-flogo/websocket/internal/testing"
	"github.com/stretchr/testify/assert"
)

var res string

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bytearr, err := ioutil.ReadAll(r.Body)
	messageToLog := fmt.Sprintf(`{"Received":"%s"}`, string(bytearr))

	if err != nil {
		panic(err)
	}
	res = messageToLog
	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(messageToLog))
	if err != nil {
		panic(err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("upgrade error", err)
	} else {
		defer conn.Close()
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read error", err)
				break
			}
			messageToLog := fmt.Sprintf("Received %s", message)
			conn.WriteMessage(mt, []byte(messageToLog))
			if err != nil {
				fmt.Println("write error", err)
				break
			}
		}
		return
	}
}

func testApplication(t *testing.T, e engine.Engine, mode string, maxconn string) {
	defer api.ClearResources()
	test.Drain("8080")
	middleware := http.NewServeMux()
	middleware.HandleFunc("/ws", wsHandler)
	middleware.HandleFunc("/pets", proxyHandler)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        middleware,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		s.ListenAndServe()
	}()
	test.Pour("8080")
	defer s.Shutdown(context.Background())

	test.Drain("9096")
	err := e.Start()
	assert.Nil(t, err)
	defer func() {
		err := e.Stop()
		assert.Nil(t, err)
	}()
	test.Pour("9096")
	if mode == "2" {
		max, _ := strconv.Atoi(maxconn)
		for i := 0; i <= max; i++ {
			conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:9096/ws", nil)
			if err != nil {
				fmt.Println("conn err", err)
				return
			}
			defer conn.Close()
			message := fmt.Sprintf(`{"%s-%v":"%v"}`, "Client1", i, 0)
			err = conn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				fmt.Println("write err", err)
				return
			}

			expectedmsg := fmt.Sprintf(`Received %s`, message)
			_, receivedMsg, err := conn.ReadMessage()
			if err != nil {
				receivedMsg = []byte(fmt.Sprintf(`websocket: close 1000 (normal): proxy service[WSProxy] utilized maximum[%s] allowed concurrent connections, can't accept any more connections`, maxconn))
				expectedmsg = string(err.Error())
			}
			assert.Equal(t, string(receivedMsg), expectedmsg)
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing client"))
			if err != nil {
				fmt.Println("close err", err)
			}
		}
	}
	if mode == "1" {
		conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:9096/ws", nil)
		if err != nil {
			fmt.Println("conn err", err)
			return
		}
		defer conn.Close()
		message := fmt.Sprintf(`{"%s-%v":"%v"}`, "Client1", 0, 0)
		err = conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			fmt.Println("write err", err)
			return
		}
		expectedmsg := fmt.Sprintf(`{"Received":"%s"}`, message)
		time.Sleep(time.Second)
		assert.Equal(t, res, expectedmsg)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing client"))
		if err != nil {
			fmt.Println("close err", err)
		}
	}
}

func TestIntegrationAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	parameters := []struct {
		mode    string
		maxConn string
	}{
		{"1", "0"}, {"2", "3"},
	}
	for i := range parameters {
		e, err := Example(parameters[i].mode, parameters[i].maxConn)
		assert.Nil(t, err)
		testApplication(t, e, parameters[i].mode, parameters[i].maxConn)
	}
}

func TestIntegrationJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping JSON integration test in short mode")
	}
	parameters := []struct {
		mode    string
		maxConn string
	}{
		{"1", "0"}, {"2", "3"},
	}
	for i := range parameters {
		data, err := ioutil.ReadFile(filepath.FromSlash("./json/mode" + parameters[i].mode + "/flogo.json"))
		assert.Nil(t, err)
		var Input input
		err = json.Unmarshal(data, &Input)
		assert.Nil(t, err)
		if parameters[i].mode == "1" {
			Input.Trig[0].Handlers[0]["settings"] = map[string]interface{}{
				"method": "GET",
				"path":   "/ws",
				"mode":   parameters[i].mode,
			}
		}
		if parameters[i].mode == "2" {
			Input.Trig[0].Handlers[0]["settings"] = map[string]interface{}{
				"method": "GET",
				"path":   "/ws",
				"mode":   parameters[i].mode,
			}
			Input.Resources[0].Data.Services[0]["settings"] = map[string]interface{}{
				"uri":            "ws://localhost:8080/ws",
				"maxconnections": parameters[i].maxConn,
			}
		}
		data, _ = json.Marshal(Input)
		cfg, err := engine.LoadAppConfig(string(data), false)
		assert.Nil(t, err)
		e, err := engine.New(cfg)
		assert.Nil(t, err)
		testApplication(t, e, parameters[i].mode, parameters[i].maxConn)
	}
}

//--------data structure-------//

type input struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Version  string      `json:"version"`
	Desc     string      `json:"description"`
	Prop     interface{} `json:"properties"`
	Channels interface{} `json:"channels"`
	Trig     []struct {
		Name     string                   `json:"name"`
		ID       string                   `json:"id"`
		Ref      string                   `json:"ref"`
		Settings interface{}              `json:"settings"`
		Handlers []map[string]interface{} `json:"handlers"`
	} `json:"triggers"`
	Resources []struct {
		ID       string `json:"id"`
		Compress bool   `json:"compressed"`
		Data     struct {
			Name      string                   `json:"name"`
			Steps     []interface{}            `json:"steps"`
			Responses []interface{}            `json:"responses"`
			Services  []map[string]interface{} `json:"services"`
		} `json:"data"`
	} `json:"resources"`
	Actions interface{} `json:"actions"`
}
