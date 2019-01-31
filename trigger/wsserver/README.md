# WebSocket Server

This trigger provides your Microgateway application with the ability to operate as a web socket server.

## Schema
Settings, Outputs and Handlers:

```json
{
 "settings":[
    {
      "name": "port",
      "type": "integer"
    },
    {
      "name": "enableTLS",
      "type": "boolean"
    },
    {
      "name": "serverCert",
      "type": "string"
    },
    {
      "name": "serverKey",
      "type": "string"
    },
    {
      "name": "enableClientAuth",
      "type": "boolean"
    },
    {
      "name": "trustStore",
      "type": "string"
    }
  ],
  "outputs": [
    {
      "name": "pathParams",
      "type": "params"
    },
    {
      "name": "queryParams",
      "type": "params"
    },
    {
      "name": "headers",
      "type": "params"
    },
    {
      "name": "content",
      "type": "any"
    },
    {
      "name": "wsconnection",
      "type": "any"
    },
  ],
  "handler": {
    "settings": [
      {
        "name": "method",
        "type": "string"
      },
      {
        "name": "path",
        "type": "string"
      },
      {
        "name": "mode",
        "type": "string"
      }
    ]
  }
}
```

### Settings
| Key    | Description   |
|:-----------|:--------------|
| port | The port to listen on |
| enableTLS | true - To enable TLS (Transport Layer Security), false - No TLS security  |
| serverCert | Server certificate file in PEM format. Need to provide file name along with path. Path can be relative to gateway binary location. |
| serverKey | Server private key file in PEM format. Need to provide file name along with path. Path can be relative to gateway binary location. |
| enableClientAuth | true - To enable client AUTH, false - Client AUTH is not enabled |
| trustStore | Trust dir containing clinet CAs |

### Outputs
| Key    | Description   |
|:-----------|:--------------|
| pathParams | HTTP request path params |
| queryParams | HTTP request query params |
| headers | HTTP request header params. Header key gets converted in to canonical format, i.e. the first letter and any letter following a hyphen to upper case, the rest are converted to lowercase. For example, the canonical key for "accept-encoding" and "host" are "Accept-Encoding" and "Host" respectively |
| content | HTTP request payload |
| wsconnection | The websocket connection |

### Handler settings
| Key    | Description   |
|:-----------|:--------------|
| method | HTTP request method. It can be |
| path | URL path to be registered with handler |
| mode | "1" for output with content and "2" for output with wsconnection |

## Example Configurations

```json
{
  "name": "tibco-wssub",
  "id": "flogo-WSMessageTrigger",
  "ref": "github.com/project-flogo/websocket/trigger/wsserver",
  "settings": {
    "port": "9096",
    "enableTLS": false,
    "serverCert": "",
    "serverKey": "",
    "enableClientAuth": false,
    "trustStore": ""
  },
  "handlers": [
    {
      "settings": {
        "method": "GET",
        "path": "/ws",
        "mode": "1"
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
