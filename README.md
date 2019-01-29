# Websocket Triggers, Activity, and Examples
This repo contains websocket related triggers, activity, and examples. It is intended to work with the [microgateway](https://github.com/project-flogo/microgateway).

## Development

### Testing

To run tests issue the following command in the root of the project:

```bash
go test -p 1 ./...
```

The `-p 1` is needed to prevent tests from being run in parallel. To re-run the tests first run the following:

```bash
go clean -testcache
```

To skip the integration tests use the `-short` flag:

```bash
go test -p 1 -short ./...
```
