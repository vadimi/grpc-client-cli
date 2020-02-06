# Description
`grpc-client-cli` is a generic `gRPC` command line client. You can call any `gRPC` service that exposes reflection endpoint.

At this point only `json` formatted requests are supported.

## Usage
Just specify a connection string to a servce in ~host:port~ format and follow instructions to select service, method and enter request message in `json` format.

`grpc-client-cli localhost:4400`

For full list of supported command line args please run `grpc-client-cli -h`.

The utility also supports authority header override.

```
grpc-client-cli localhost:5050,authority=localhost:9090
```

### Subcommands
**discover** - print servive proto contract

```
grpc-client-cli discover localhost:5050
grpc-client-cli -s User discover localhost:5050
```

**health** - call health check service

```
grpc-client-cli health localhost:5050
```

### Non-interactive mode
In non-interactive mode `grpc-client-cli` expects all parameters to be passed to execute gRPC service.

**Pass message json through stdin**
```
echo '{"user_id": "12345"}' | grpc-client-cli -service UserService -method GetUser localhost:5050
```

```
cat message.json | grpc-client-cli -service UserService -method GetUser localhost:5050
```

On windows this could be achieved using `type` command
```
type message.json | grpc-client-cli -service UserService -method GetUser localhost:5050
```

**Input file**

Another option of providing a file with message json is `-input` (or `-i`) parameter:

```
grpc-client-cli -service UserService -method GetUser -i message.json localhost:5050
```

## JSON format specifics
Most of the fields in proto message can be intuitively mapped to `json` types. There are some exclusions though:

1. `Timestamp` mapped to a string in `ISO 8601` format.

For example:
```
{
  "flight_start_date": "2018-03-19T00:00:00.0Z"
}
```

2. `Duration` mapped to a string in the following format: `00h00m00s`

For example:
```
{
  "start_time": "20h00m00s",
  "some_other_duration": "1s"
}
```
