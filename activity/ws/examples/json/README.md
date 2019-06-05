# HTTP to WebSocket proxy
This recipe sends messages to a websocket

## Installation
* Install [Go](https://golang.org/)

To install run the following commands:

```bash
flogo create -f flogo.json
cd MyProxy
flogo build
```

## Testing
Start server:
```bash
go run main.go -server
```

Start the gateway in MyProxy directory:
```bash
bin/MyProxy
```

Run:
```bash
curl -H "Content-Type: application/json" -d '{"message": "hello world"}' http://localhost:9096/message
```

You should see in the server terminal:
```
Received message({"message":"hello world"}) from the client(127.0.0.1:47890)
```
