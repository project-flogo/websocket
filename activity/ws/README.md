# WebSocket Client

This activity accepts a message which is sent to a websocket server.

The service `settings` for the request are as follows:

| Name   |  Type   | Description   |
|:-----------|:--------|:--------------|
| uri | string | Backend websocket uri to connect |

Available `input` for the request are as follows:

| Name   |  Type   | Description   |
|:-----------|:--------|:--------------|
| message | message object | A message to send |

A sample `service` definition is:

```json
{
    "name": "Websocket",
    "description": "Web socket sending service",
    "ref": "github.com/project-flogo/websocket/activity/ws",
    "settings":{
        "uri": "ws://localhost:8080/ws"
    }
}
```

An example `step` that invokes the above `Websocket` service using `message` is:

```json
{
    "service": "Websocket",
    "input": {
      "message":"=$.payload.content"
    }
}
```
