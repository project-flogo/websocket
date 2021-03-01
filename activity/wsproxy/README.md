# WebSocket Proxy

The `ws` service type accepts a websocket connection and backend url. It establishes another websocket connection against supplied backend url and acts as a proxy between both connections. This service doesn't produce any outputs as it runs proxy instance in the background and returns immediately.

The service `settings` for the request are as follows:

| Name   |  Type   | Description   |
|:-----------|:--------|:--------------|
| uri | string | Backend websocket uri to connect |
| maxConnections | number | Maximum allowed concurrent connections(default 5) |

Available `input` for the request are as follows:

| Name   |  Type   | Description   |
|:-----------|:--------|:--------------|
| wsconnection | connection object | Websocket connection object |

A sample `service` definition is:

```json
{
    "name": "ProxyWebSocketService",
    "description": "Web socket proxy service",
    "ref": "github.com/project-flogo/websocket/activity/wsproxy",
    "settings":{
        "uri": "ws://localhost:8080/ws",
        "maxConnections": 5
    }
}
```

An example `step` that invokes the above `ProxyWebSocketService` service using `wsconnection` is:

```json
{
    "service": "ProxyWebSocketService",
    "input": {
        "wsconnection":"=$.payload.wsconnection"
    }
}
```
