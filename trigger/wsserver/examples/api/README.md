# Gateway with a WebSocket
This recipe is a gateway with a service through websocket.

## Installation
* Install [Go](https://golang.org/)

## Testing
Default values:
```
mode = 2
maxconnections = 3

Mode: Values(1 or 2). Mode 1 is for getting message from client and sending it to action. Mode 2 is for streaming messages
MaxNumberofconnections: This is required for mode 2. It decides the limit for client connections
```
and test below scenario.

# MODE 1 : Receive message from client and run the associated action
Start the gateway:
```bash
go run main.go -mode 1
```

Run:

Step 1: Start server
```bash
go run main.go -server
```

Then open another terminal and run client:
Step 2:
```bash
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

You should then see something like on server screen after equal intervals
Received message({"CLIENT4-4":"1543878185"}) from the client ({client name + message count: timestamp})
The server runs the action and on the trigger screen you can see the service being invoked


# MODE 2: Receive connection and send it to server
Start the gateway:
```bash
go run main.go -maxconn 2
```

Run:

Step 1: Start server
```bash
go run main.go -server
```

Then open another terminal and run client:
Step 2:
```bash
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

Run 2nd Client:
```bash
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

You should then see something like on server screen after equal intervals
Received message({"CLIENT4-4":"1543878185"}) from the client ({client name + message count: timestamp})
from all the client connections

Eg: we set maxconnections = 2
Now you should see that gateway rejecting 3rd client connection.
You can change maximum allowed concurrent connections using maxConnections service setting.

On Running 3rd client:
```bash
go run main.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

We see the message:
Read error websocket: close 1000 (normal): proxy service[ProxyWebSocketService] utilized maximum[2]
allowed concurrent connections, can't accept any more connections
