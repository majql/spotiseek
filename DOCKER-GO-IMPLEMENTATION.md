# Docker Go SDK Implementation Guide

## Required Dependencies

### Core Docker Package
```bash
go get github.com/docker/docker@latest
```

### Additional Required Dependencies
```bash
go get github.com/docker/go-connections/nat
go get github.com/docker/go-units
go get github.com/moby/docker-image-spec/specs-go/v1
go get github.com/opencontainers/image-spec/specs-go/v1
go get github.com/opencontainers/go-digest
go get github.com/containerd/errdefs
go get github.com/containerd/errdefs/pkg/errhttp
go get github.com/distribution/reference
go get github.com/gogo/protobuf/proto
go get github.com/docker/go-connections/sockets
go get github.com/docker/go-connections/tlsconfig
go get github.com/pkg/errors
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
go get go.opentelemetry.io/otel/trace
```

## Required Imports
```go
import (
    "context"
    "fmt"
    "io"
    "log"
    "time"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)
```

## Client Initialization
```go
cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
if err != nil {
    log.Fatal("Failed to create Docker client:", err)
}
defer cli.Close()
```

**Important**: Use `client.WithAPIVersionNegotiation()` to automatically negotiate compatible API version.

## Image Operations

### Pull Image
```go
reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
if err != nil {
    log.Fatal("Failed to pull image:", err)
}
// Must read the response stream completely
_, err = io.Copy(io.Discard, reader)
if err != nil {
    log.Fatal("Failed to read pull response:", err)
}
reader.Close()
```

**Important**: Always read the pull response stream with `io.Copy()` to ensure the pull completes properly.

## Container Operations

### Create Container
```go
containerConfig := &container.Config{
    Image: imageName,
    Cmd:   []string{"sleep", "30"},
}

resp, err := cli.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
if err != nil {
    log.Fatal("Failed to create container:", err)
}
containerID := resp.ID
```

### Start Container
```go
err = cli.ContainerStart(ctx, containerID, container.StartOptions{})
if err != nil {
    log.Fatal("Failed to start container:", err)
}
```

### Stop Container
```go
timeout := 30
err = cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
if err != nil {
    log.Fatal("Failed to stop container:", err)
}
```

### Remove Container
```go
err = cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})
if err != nil {
    log.Fatal("Failed to remove container:", err)
}
```

## Common Patterns

### Basic Container Lifecycle
1. Pull image (if needed)
2. Create container with configuration
3. Start container
4. Wait/perform operations
5. Stop container
6. Remove container

### Error Handling
- Always check errors after each Docker operation
- Use `log.Fatal()` for critical errors that should stop execution
- Consider cleanup in defer statements for production code

## Context7 Library ID
- Use `/moby/moby` for Docker Go SDK documentation
- The Docker SDK is part of the Moby project

## Notes
- The Docker daemon API version compatibility is handled automatically with `client.WithAPIVersionNegotiation()`
- Always close resources (client, readers) using defer statements
- Image pulling requires reading the response stream to completion