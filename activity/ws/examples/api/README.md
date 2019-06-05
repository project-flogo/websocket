# HTTP to WebSocket proxy
This recipe sends messages to a websocket

## Installation
* Install [Go](https://golang.org/)

## Testing
Start server:
```bash
go run main.go -server
```

Start the gateway:
```bash
go run main.go
```

Run:
```bash
curl -H "Content-Type: application/json" -d '{"message": "hello world"}' http://localhost:9096/message
```

You should see in the server terminal:
```
Received message({"message":"hello world"}) from the client(127.0.0.1:47890)
```
