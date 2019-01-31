# Install
To install run the following commands:

flogo create -f flogo.json
cd wssub
flogo build

# Testing
Run:

Step 1: Run Server
go run helper.go -server

Then open another terminal
Step 2: Start wssub trigger

Then open another terminal and run client:
Step 3:
go run helper.go -client


You should then see something like on trigger screen after equal intervals
Message received : CLIENTNAME-1-1543273633 (client name + message count + timestamp)
