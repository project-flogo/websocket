# WebSocket Client
This trigger provides your Microgateway application with the ability to subscribe to websocket message events and invokes `dispatch` with the contents of the message.

## Schema
Settings, Outputs and Handlers:

```json
{
 "settings":[
    {
      "name": "url",
      "type": "string"
    }
  ],
  "outputs": [
    {
      "name": "content",
      "type": "any"
    }
  ],
  "handler": {
    "settings": []
  }
}
```

### Settings
| Key    | Description   |
|:-----------|:--------------|
| url | The websocket url to connect to. |

### Outputs
| Key    | Description   |
|:-----------|:--------------|
| content | Websocket request payload |

## Example Configurations

```json
{
  "name": "tibco-wssub",
  "id": "flogo-WSMessageTrigger",
  "ref": "github.com/project-flogo/websocket/trigger/wsclient",
  "settings": {
    "url": "ws://localhost:8000/ws"
  },
  "handlers": [
    {
      "settings": {
      },
      "actions": [
        {
          "id": "microgateway:Pets"
        }
      ]
    }
  ]
}
```
