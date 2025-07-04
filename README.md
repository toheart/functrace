# FuncTrace - Go Function Tracing and Performance Analysis Library

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.19-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

FuncTrace is a comprehensive Go library designed for tracking and analyzing function calls in Go applications. Built with Domain-Driven Design (DDD) architecture, it provides detailed insights into function execution patterns, performance metrics, and goroutine lifecycles.

**[中文文档 / Chinese Documentation](README_zh.md)**

## Features

### 🔍 Function Call Tracing
- **Decorator Pattern**: Automatic function entry/exit tracking with simple decorator syntax
- **Call Chain Analysis**: Complete parent-child relationship mapping for nested function calls
- **Execution Timing**: Precise CPU execution time measurement for each function
- **Hierarchical Display**: Automatic indentation based on call depth for clear visualization

### 📊 Parameter Storage System
Three flexible parameter storage modes to balance functionality and memory usage:

#### `none` Mode (Default - Memory Efficient)
- Records only function call chains and execution times
- Minimal memory footprint, ideal for production environments
- Best for performance monitoring without detailed debugging

#### `normal` Mode (Balanced)
- Captures parameters for regular functions and value receiver methods
- Moderate memory usage, suitable for development environments
- Good balance between debugging capability and resource consumption

#### `all` Mode (Complete Debugging)
- Records all parameters including complex object changes
- Uses JSON Patch technology for incremental storage of pointer receiver changes, greatly reducing redundant data and improving efficiency for large object tracking
- Highest memory usage, ideal for detailed problem analysis
- Includes built-in memory protection mechanisms

### 🚀 Goroutine Monitoring
- **Real-time Tracking**: Monitor creation, execution, and termination of goroutines
- **Lifecycle Management**: Automatic recording of total goroutine execution times
- **Background Cleanup**: Periodic background tasks to clean up finished goroutine traces
- **State Synchronization**: Thread-safe goroutine state management
- **Main Exit Data Safety**: On `main.main` exit, automatically waits for all trace data to be persisted, ensuring data integrity

### 🛡️ Memory Protection
- **Memory Monitor**: Automatic memory usage monitoring in `all` mode
- **Threshold Protection**: Default 2GB memory limit with emergency exit to prevent OOM
- **Smart Alerts**: Clear error messages and solution suggestions
- **Configurable Limits**: Customizable memory thresholds via environment variables

### 💾 Data Persistence
Repository pattern supporting multiple storage backends:

#### SQLite Storage (Default)
- Three main tables: `TraceData`, `GoroutineTrace`, `ParamStoreData`
- Support for both synchronous and asynchronous insertion modes
- Automatic index creation for optimized query performance
- WAL mode for improved concurrent access

#### Memory Storage
- Mock implementation for testing purposes
- High-speed in-memory operations
- Perfect for unit testing and development

### 🔧 Intelligent Parameter Serialization
Enhanced spew package with:
- **JSON Output**: Structured JSON format for complex objects
- **Memory Pool Optimization**: Object pooling to reduce memory allocation overhead
- **Type Safety**: Safe handling of all Go data types including unsafe operations
- **Circular Reference Detection**: Prevention of infinite recursion and stack overflow
- **Advanced Type Support**: Now supports interface, pointer, and byte array types
- **Improved MaxDepth Truncation**: Truncation output now includes detailed metadata (`__truncated__`, `num_fields`, `length`, `type`) for easier debugging

## Installation

```bash
go get github.com/toheart/functrace
```

## Quick Start

### Basic Usage

```go
package main

import (
    "time"
    "github.com/toheart/functrace"
)

func ExampleFunction(name string, count int) {
    defer functrace.Trace([]interface{}{name, count})()
    
    // Your function logic here
    for i := 0; i < count; i++ {
        processItem(name, i)
    }
}

func processItem(name string, index int) {
    defer functrace.Trace([]interface{}{name, index})()
    
    // Processing logic
    time.Sleep(10 * time.Millisecond)
}

func main() {
    defer functrace.CloseTraceInstance()
    
    ExampleFunction("test", 3)
}
```

### Advanced Configuration

```go
package main

import (
    "os"
    "github.com/toheart/functrace"
)

func main() {
    // Configure parameter storage mode
    os.Setenv("FUNCTRACE_PARAM_STORE_MODE", "normal")
    
    // Configure async database operations
    os.Setenv("ENV_DB_INSERT_MODE", "async")
    
    // Configure memory limit (2GB)
    os.Setenv("FUNCTRACE_MEMORY_LIMIT", "2147483648")
    
    defer functrace.CloseTraceInstance()
    
    // Your application logic
    YourApplicationLogic()
}
```

## Configuration

FuncTrace supports configuration through environment variables:

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `FUNCTRACE_PARAM_STORE_MODE` | `none` | Parameter storage mode: `none`/`normal`/`all` |
| `ENV_DB_INSERT_MODE` | `sync` | Database insertion mode: `sync`/`async` |
| `FUNCTRACE_MEMORY_LIMIT` | `2147483648` | Memory limit in bytes (2GB default) |
| `FUNCTRACE_IGNORE_NAMES` | `log,context,string` | Comma-separated function name keywords to ignore |
| `FUNCTRACE_GOROUTINE_MONITOR_INTERVAL` | `10` | Goroutine monitoring interval in seconds |
| `FUNCTRACE_MAX_DEPTH` | `3` | Maximum tracing depth |

## Parameter Storage Modes Comparison

| Mode | Memory Usage | Features | Use Case |
|------|-------------|----------|----------|
| `none` | Minimal | Function call chains + execution times | test monitoring |
| `normal` | Moderate | Regular function parameters + value methods | Development debugging |
| `all` | High | All parameters + pointer receiver diffs | Detailed problem analysis |

## Database Schema

### TraceData Table
- `id`: Unique identifier
- `name`: Function name
- `gid`: Goroutine ID
- `indent`: Indentation level
- `paramsCount`: Number of parameters
- `timeCost`: CPU execution time
- `parentId`: Parent function ID
- `createdAt`: Creation timestamp
- `isFinished`: Completion status
- `seq`: Sequence number

### GoroutineTrace Table
- `id`: Auto-increment ID
- `originGid`: Original Goroutine ID
- `timeCost`: CPU execution time
- `createTime`: Creation time
- `isFinished`: Completion status
- `initFuncName`: Initial function name

### ParamStoreData Table
- `id`: Unique identifier
- `traceId`: Associated TraceData ID
- `position`: Parameter position
- `data`: Parameter JSON data
- `isReceiver`: Whether it's a receiver parameter
- `baseId`: Base parameter ID (for incremental storage)

## Architecture

FuncTrace follows a clean layered architecture:

```
API Layer (functrace.go)
    ↓
Core Layer (trace package)
    ↓
Domain Layer (domain package)
    ↓
Persistence Layer (persistence package)
```

### Key Components

- **API Layer**: Simple external interface (`functrace.go`)
- **Core Layer**: Main tracing logic (`trace/`)
- **Domain Layer**: Business models and repository interfaces (`domain/`)
- **Persistence Layer**: Data storage implementations (`persistence/`)

## Performance Considerations

### Memory Optimization
- Object pooling for reduced garbage collection
- Configurable memory limits with automatic protection
- Efficient JSON serialization with incremental storage

### Database Optimization
- Asynchronous insertion mode for high-throughput scenarios
- Proper indexing for fast queries
- Connection pooling and WAL mode for SQLite

### Concurrency Safety
- Thread-safe goroutine state management
- Lock-free atomic operations where possible
- Proper synchronization for shared data structures

## Best Practices

1. **Production Use**: Use `none` parameter mode with `async` database mode
2. **Development**: Use `normal` parameter mode for balanced debugging
3. **Deep Debugging**: Use `all` parameter mode with memory monitoring
4. **Resource Management**: Always call `functrace.CloseTraceInstance()` before exit
5. **Selective Tracing**: Use ignore patterns to exclude frequently called functions

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [Wiki](https://github.com/toheart/functrace/wiki)
- **Issues**: [GitHub Issues](https://github.com/toheart/functrace/issues)
- **Discussions**: [GitHub Discussions](https://github.com/toheart/functrace/discussions)

## Acknowledgments

- Built with [spew](https://github.com/davecgh/go-spew) for advanced data serialization
- Uses [SQLite](https://sqlite.org/) for efficient data persistence
- Inspired by various Go profiling and tracing tools

## Testing & Coverage

All core features are covered by unit tests (target coverage 80%+). Use `go test -cover` to check coverage.