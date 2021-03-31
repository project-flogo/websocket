# Install
To install run the following commands:

```bash
flogo create -f flogo.json
cd wstrigger
flogo build
```

Repeat above procces for flogo.json in both mode1 and mode2 directories.

# Testing
## MODE 1 : Receive message from client and run the associated action
Using the flogo application built in mode1 directory run:

Step 1: Start ws- trigger
```bash
wstrigger/bin/wstrigger
```

Step 2: Run Server
```bash
go run helper.go -server
```

Then open another terminal and run client:
Step 3:
```bash
go run helper.go -client -name=<client_name> -url=ws://localhost:9096/ws
```


You should then see something like on server screen after equal intervals
Received message({"CLIENT4-4":"1543878185"}) from the client ({client name + message count: timestamp})
The server runs the action and on the trigger screen you can see 200 (success code)

## MODE 2: Receive connection and send it to server
Using the flogo application built in mode2 directory run:

Step 1: Start ws- trigger
```bash
wstrigger/bin/wstrigger
```

Step 2: Run Server
```bash
go run helper.go -server
```

Then open another terminal and run client:
Step 3:
```bash
go run helper.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

Run 2nd Client:
```bash
go run helper.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

You should then see something like on server screen after equal intervals
Received message({"CLIENT4-4":"1543878185"}) from the client ({client name + message count: timestamp})
from all the client connections

Here, maxconnections = 2
Now you should see that gateway rejecting 3rd client connection.
You can change maximum allowed concurrent connections using maxConnections service setting.

On Running 3rd client:
```bash
go run helper.go -client -name=<client_name> -url=ws://localhost:9096/ws
```

We see the message:
Read error websocket: close 1000 (normal): proxy service[ProxyWebSocketService] utilized maximum[2]
allowed concurrent connections, can't accept any more connections
