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

func testApplication(t *testing.T, e engine.Engine, maxConn string) {
	defer api.ClearResources()
	test.Drain("8080")
	middleware := http.NewServeMux()
	middleware.HandleFunc("/ws", wsHandler)
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
		assert.Nil(t, e.Stop())
	}()
	test.Pour("9096")

	max, err := strconv.Atoi(maxConn)
	assert.Nil(t, err)
	for i := 0; i <= max; i++ {
		conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:9096/ws", nil)
		if err != nil {
			fmt.Println("conn err", err)
			return
		}
		message := fmt.Sprintf(`{"%s-%v": "%v"}`, "Client1", i, 0)
		err = conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			fmt.Println("write err", err)
			conn.Close()
			return
		}
		expectedmsg := fmt.Sprintf(`Received %s`, message)
		_, receivedMsg, err := conn.ReadMessage()
		if err != nil {
			receivedMsg = []byte(fmt.Sprintf(`websocket: close 1000 (normal): proxy service[WSProxy] utilized maximum[%s] allowed concurrent connections, can't accept any more connections`, maxConn))
			expectedmsg = string(err.Error())
		}
		assert.Equal(t, string(receivedMsg), expectedmsg)
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing client"))
		if err != nil {
			fmt.Println("close err", err)
		}
		conn.Close()
	}
}

func TestIntegrationAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	parameters := []struct {
		maxconnections string
	}{
		{"2"}, {"4"},
	}
	for i := range parameters {
		e, err := Example(parameters[i].maxconnections)
		assert.Nil(t, err)
		testApplication(t, e, parameters[i].maxconnections)
	}
}

func TestIntegrationJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping JSON integration test in short mode")
	}
	parameters := []struct {
		maxconnections string
	}{
		{"2"}, {"4"},
	}
	data, err := ioutil.ReadFile(filepath.FromSlash("./json/flogo.json"))
	assert.Nil(t, err)
	for i := range parameters {
		var Input input
		err = json.Unmarshal(data, &Input)
		assert.Nil(t, err)
		Input.Resources[0].Data.Services[0]["settings"] = map[string]interface{}{
			"maxconnections": parameters[i].maxconnections,
			"uri":            "ws://localhost:8080/ws",
		}
		data, err = json.Marshal(Input)
		assert.Nil(t, err)
		cfg, err := engine.LoadAppConfig(string(data), false)
		assert.Nil(t, err)
		e, err := engine.New(cfg)
		assert.Nil(t, err)
		testApplication(t, e, parameters[i].maxconnections)
	}
}

//--------data structure-------//

type input struct {
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Version   string      `json:"version"`
	Desc      string      `json:"description"`
	Prop      interface{} `json:"properties"`
	Channels  interface{} `json:"channels"`
	Trig      interface{} `json:"triggers"`
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
