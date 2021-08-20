# Description

`grpc-client-cli` is a generic `gRPC` command line client - call any `gRPC` service. If your service exposes [gRPC Reflection service](https://github.com/grpc/grpc-proto/blob/master/grpc/reflection/v1/reflection.proto) the tool will discover all services and methods automatically. If not, please specify `--proto` parameter with the path to proto files.

![](images/demo.gif)

## Installation

Download the binary and install it to `/usr/local` directory:

- `curl -L https://github.com/vadimi/grpc-client-cli/releases/download/v1.10.0/grpc-client-cli_darwin_x86_64.tar.gz | tar -C /usr/local/bin -xz`

For go `1.16+` use this command to install the app to `$GOPATH/bin` directory:

- `go install github.com/vadimi/grpc-client-cli/cmd/grpc-client-cli@v1.10.0`

Or use "go get" approach:

- `GO111MODULE=on go get -u github.com/vadimi/grpc-client-cli/cmd/grpc-client-cli@latest`
- `GO111MODULE=on go get -u github.com/vadimi/grpc-client-cli/cmd/grpc-client-cli@v1.10.0`

## Usage

Just specify a connection string to a servce in `host:port` format and follow instructions to select service, method and enter request message in `json` or `proto` text format.

`grpc-client-cli localhost:4400`

In this case the service needs to expose gRPC Reflection service.

For full list of supported command line args please run `grpc-client-cli -h`.

To provide the list of services to call specify `--proto` parameter and `--protoimports` in case an additional directory for imports is required:

```
grpc-client-cli --proto /path/to/proto/files localhost:5050
```

The tool also supports `:authority` header override.

```
grpc-client-cli --authority localhost:9090 localhost:5050
```

It's also possible to capture some of the diagnostic information like request and response sizes, call duration:

```
grpc-client-cli -V localhost:4400
```

Proto text format for input and output:

```
grpc-client-cli --informat text --outformat text localhost:5050
```

### Eureka Support

grpc-client-cli provides integrated support for services published to a Eureka service registry.

Connecting to a service published to Eureka running on http://localhost:8761/eureka/

```
grpc-client-cli eureka://application-name/
```

Connecting to a service running remotely on http://example.com:8761/eureka/

```
grpc-client-cli eureka://example.com/eureka/application-name/
```

Connecting to a service running remotely on http://example.com:9000/not-eureka/

```
grpc-client-cli eureka://example.com:9000/not-eureka/application-name/
```

The Eureka currently connects to services using the IP Addresses published in the service registry and the following published ports, in order:

- Metadata key "grpc"
- Metadata key "grpc.port"
- Default insecure port

If you require a different default port, please file an issue, and that port will be considered for inclusion.

### Subcommands

**discover** - print service protobuf contract

```
grpc-client-cli discover localhost:5050
grpc-client-cli -s User discover localhost:5050
```

**health** - call [health check service](https://github.com/grpc/grpc-proto/blob/master/grpc/health/v1/health.proto), this command returns non-zero exit code in case health check returns `NOT_SERVING` response or the call fails for any other reason, so it's useful for example in kubernetes health probes

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

### Autocompletion

To enable autocompletion in your terminal add the following commands to your `.bashrc` or `.zshrc` files.

**ZSH**

```
PROG=grpc-client-cli
_CLI_ZSH_AUTOCOMPLETE_HACK=1
source  autocomplete/zsh_autocomplete
```

**Bash**

```
PROG=grpc-client-cli
source autocomplete/bash_autocomplete
```

`autocomplete` directory is located in the root of the repo. Please find more details [here](https://github.com/urfave/cli/blob/master/docs/v2/manual.md#bash-completion).

## JSON format specifics

Most of the fields in proto message can be intuitively mapped to `json` types. There are some exclusions though:

1. `Timestamp` mapped to a string in `ISO 8601` format.

For example:

```json
{
  "flight_start_date": "2018-03-19T00:00:00.0Z"
}
```

2. `Duration` mapped to a string in the following format: `00h00m00s`

For example:

```json
{
  "start_time": "20h00m00s",
  "some_other_duration": "1s"
}
```
