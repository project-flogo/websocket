# Gateway with a WebSocket
This recipe is a gateway with a service through websocket.

## Installation
* Install [Go](https://golang.org/)

## Testing
Start the gateway:
```
go run main.go
```

Testing
Run:

Step 1: Start server
go run main.go -server

Then open another terminal and run client:
Step 2:
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws


Run 2nd Client:
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws

You should then see something like on server screen after equal intervals
Received message({"CLIENT4-4":"1543878185"}) from the client ({client name + message count: timestamp})
from all the client connections

Eg: we set maxconnections = 2
Now you should see that gateway rejecting 3rd client connection.
You can change maximum allowed concurrent connections using maxConnections service setting.

On Running 3rd client:
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws

We see the message:
Read error websocket: close 1000 (normal): proxy service[ProxyWebSocketService] utilized maximum[2]
allowed concurrent connections, can't accept any more connections
