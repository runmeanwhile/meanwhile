package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type engineRef interface {
	ToolRegistry() *tool.Registry
	MCPRegistry() *Registry
}

type commandSpec struct {
	path string
	args []string
	env  map[string]string
	dir  string
}

type ioSpec struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

// Builder provides a fluent API for connecting to MCP servers.
type Builder struct {
	engine            engineRef
	name              string
	prefix            string
	toolIDFunc        ToolIDFunc
	filter            ToolFilter
	allowlist         map[string]struct{}
	denylist          map[string]struct{}
	implementation    sdkmcp.Implementation
	clientOptions     sdkmcp.ClientOptions
	sessionOptions    *sdkmcp.ClientSessionOptions
	autoRefresh       bool
	command           *commandSpec
	commandExec       *exec.Cmd
	streamableURL     string
	sseURL            string
	ioSpec            *ioSpec
	customTransport   sdkmcp.Transport
	headers           http.Header
	httpClient        *http.Client
	streamableRetries int
	terminateDuration time.Duration
	toolRegistry      *tool.Registry
}

// NewBuilder creates an MCP builder for the given server name.
func NewBuilder(engine engineRef, name string) *Builder {
	b := &Builder{
		engine:         engine,
		name:           name,
		prefix:         name,
		allowlist:      make(map[string]struct{}),
		denylist:       make(map[string]struct{}),
		autoRefresh:    true,
		headers:        make(http.Header),
		implementation: sdkmcp.Implementation{Name: "meanwhile", Version: "dev"},
	}
	return b
}

// Implementation sets the MCP client implementation name and version.
func (b *Builder) Implementation(name, version string) *Builder {
	if name != "" {
		b.implementation.Name = name
	}
	if version != "" {
		b.implementation.Version = version
	}
	return b
}

// Prefix sets the tool ID prefix used for MCP tools.
func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

// ToolID maps MCP tool names to local tool IDs.
func (b *Builder) ToolID(fn ToolIDFunc) *Builder {
	b.toolIDFunc = fn
	return b
}

// AllowTools limits exposure to the provided tool names.
func (b *Builder) AllowTools(names ...string) *Builder {
	for _, name := range names {
		if name == "" {
			continue
		}
		b.allowlist[name] = struct{}{}
	}
	return b
}

// DenyTools blocks the provided tool names.
func (b *Builder) DenyTools(names ...string) *Builder {
	for _, name := range names {
		if name == "" {
			continue
		}
		b.denylist[name] = struct{}{}
	}
	return b
}

// Filter applies a custom tool filter.
func (b *Builder) Filter(filter ToolFilter) *Builder {
	if filter == nil {
		return b
	}
	if b.filter == nil {
		b.filter = filter
		return b
	}
	prev := b.filter
	b.filter = func(tool *sdkmcp.Tool) bool {
		return prev(tool) && filter(tool)
	}
	return b
}

// AutoRefresh toggles tool refresh on tool list changed notifications.
func (b *Builder) AutoRefresh(enabled bool) *Builder {
	b.autoRefresh = enabled
	return b
}

// ClientOptions overrides MCP client options.
func (b *Builder) ClientOptions(opts sdkmcp.ClientOptions) *Builder {
	b.clientOptions = opts
	return b
}

// SessionOptions sets MCP session options.
func (b *Builder) SessionOptions(opts *sdkmcp.ClientSessionOptions) *Builder {
	b.sessionOptions = opts
	return b
}

// KeepAlive configures periodic client pinging.
func (b *Builder) KeepAlive(interval time.Duration) *Builder {
	b.clientOptions.KeepAlive = interval
	return b
}

// Command configures a command transport.
func (b *Builder) Command(path string, args ...string) *Builder {
	spec := b.ensureCommandSpec()
	spec.path = path
	spec.args = args
	b.commandExec = nil
	b.streamableURL = ""
	b.sseURL = ""
	b.ioSpec = nil
	b.customTransport = nil
	return b
}

// CommandExec uses a preconfigured exec.Cmd for the command transport.
func (b *Builder) CommandExec(cmd *exec.Cmd) *Builder {
	if cmd == nil {
		return b
	}
	b.command = nil
	b.commandExec = cmd
	b.streamableURL = ""
	b.sseURL = ""
	b.ioSpec = nil
	b.customTransport = nil
	return b
}

// Env sets an environment variable for command transports.
func (b *Builder) Env(key, value string) *Builder {
	if key == "" {
		return b
	}
	spec := b.ensureCommandSpec()
	spec.env[key] = value
	return b
}

// Dir sets the working directory for command transports.
func (b *Builder) Dir(dir string) *Builder {
	spec := b.ensureCommandSpec()
	spec.dir = dir
	return b
}

// TerminateAfter controls how long to wait before SIGTERM when closing a command transport.
func (b *Builder) TerminateAfter(duration time.Duration) *Builder {
	b.terminateDuration = duration
	return b
}

// Streamable configures a streamable HTTP transport.
func (b *Builder) Streamable(endpoint string) *Builder {
	b.streamableURL = endpoint
	b.commandExec = nil
	b.sseURL = ""
	b.command = nil
	b.ioSpec = nil
	b.customTransport = nil
	return b
}

// SSE configures the legacy SSE transport.
func (b *Builder) SSE(endpoint string) *Builder {
	b.sseURL = endpoint
	b.commandExec = nil
	b.streamableURL = ""
	b.command = nil
	b.ioSpec = nil
	b.customTransport = nil
	return b
}

// IO configures an IO transport using explicit reader/writer streams.
func (b *Builder) IO(reader io.ReadCloser, writer io.WriteCloser) *Builder {
	b.ioSpec = &ioSpec{reader: reader, writer: writer}
	b.commandExec = nil
	b.command = nil
	b.streamableURL = ""
	b.sseURL = ""
	b.customTransport = nil
	return b
}

// Transport sets a custom MCP transport.
func (b *Builder) Transport(t sdkmcp.Transport) *Builder {
	b.customTransport = t
	b.commandExec = nil
	b.command = nil
	b.streamableURL = ""
	b.sseURL = ""
	b.ioSpec = nil
	return b
}

// HTTPClient sets the HTTP client used by HTTP-based transports.
func (b *Builder) HTTPClient(client *http.Client) *Builder {
	b.httpClient = client
	return b
}

// Header adds a header to HTTP-based transports.
func (b *Builder) Header(key, value string) *Builder {
	if key == "" {
		return b
	}
	b.headers.Add(key, value)
	return b
}

// Headers adds multiple headers to HTTP-based transports.
func (b *Builder) Headers(headers http.Header) *Builder {
	for key, values := range headers {
		for _, value := range values {
			b.headers.Add(key, value)
		}
	}
	return b
}

// StreamableRetries sets the max retries for streamable HTTP transports.
func (b *Builder) StreamableRetries(max int) *Builder {
	b.streamableRetries = max
	return b
}

// RegisterTo sets the tool registry to register remote tools into.
func (b *Builder) RegisterTo(reg *tool.Registry) *Builder {
	b.toolRegistry = reg
	return b
}

// Connect establishes a connection to the MCP server.
func (b *Builder) Connect(ctx context.Context) (*Server, error) {
	server, err := b.buildServer()
	if err != nil {
		return nil, err
	}
	if err := server.Connect(ctx); err != nil {
		return nil, err
	}

	b.registerServer(server)
	return server, nil
}

// Register connects to the server, loads tools, and registers them with the tool registry.
func (b *Builder) Register(ctx context.Context) (*Server, error) {
	server, err := b.Connect(ctx)
	if err != nil {
		return nil, err
	}
	reg := b.toolRegistry
	if reg == nil && b.engine != nil {
		reg = b.engine.ToolRegistry()
	}
	if reg != nil {
		server.RegisterTools(reg)
	}
	if err := server.Refresh(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (b *Builder) buildServer() (*Server, error) {
	transportFactory := b.transportFactory()
	if transportFactory == nil {
		return nil, ErrTransportRequired
	}

	options := b.clientOptions
	userHandler := options.ToolListChangedHandler

	var server *Server
	if b.autoRefresh {
		options.ToolListChangedHandler = func(ctx context.Context, req *sdkmcp.ToolListChangedRequest) {
			if server != nil {
				_ = server.Refresh(ctx)
			}
			if userHandler != nil {
				userHandler(ctx, req)
			}
		}
	} else if userHandler != nil {
		options.ToolListChangedHandler = userHandler
	}

	client := sdkmcp.NewClient(&b.implementation, &options)
	allowlist := make(map[string]struct{}, len(b.allowlist))
	for name := range b.allowlist {
		allowlist[name] = struct{}{}
	}
	denylist := make(map[string]struct{}, len(b.denylist))
	for name := range b.denylist {
		denylist[name] = struct{}{}
	}
	server = &Server{
		name:            b.name,
		prefix:          b.prefix,
		toolIDFunc:      b.toolIDFunc,
		filter:          b.filter,
		allowlist:       allowlist,
		denylist:        denylist,
		client:          client,
		transportSource: transportFactory,
		sessionOptions:  b.sessionOptions,
		tools:           make(map[string]*ProxyTool),
		toolNames:       make(map[string]*ProxyTool),
	}
	return server, nil
}

func (b *Builder) registerServer(server *Server) {
	if b.engine == nil {
		return
	}
	reg := b.engine.MCPRegistry()
	if reg == nil {
		return
	}
	reg.Register(server)
}

func (b *Builder) transportFactory() transportFactory {
	if b.customTransport != nil {
		transport := b.customTransport
		return func() (sdkmcp.Transport, error) {
			return transport, nil
		}
	}

	if b.commandExec != nil {
		cmd := b.commandExec
		return func() (sdkmcp.Transport, error) {
			return &sdkmcp.CommandTransport{Command: cmd, TerminateDuration: b.terminateDuration}, nil
		}
	}

	if b.command != nil {
		spec := *b.command
		return func() (sdkmcp.Transport, error) {
			if spec.path == "" {
				return nil, fmt.Errorf("command path required")
			}
			cmd := exec.Command(spec.path, spec.args...)
			cmd.Env = mergeEnv(os.Environ(), spec.env)
			if spec.dir != "" {
				cmd.Dir = spec.dir
			}
			return &sdkmcp.CommandTransport{Command: cmd, TerminateDuration: b.terminateDuration}, nil
		}
	}

	if b.streamableURL != "" {
		endpoint := b.streamableURL
		retries := b.streamableRetries
		client := withHeaders(b.httpClient, b.headers)
		return func() (sdkmcp.Transport, error) {
			return &sdkmcp.StreamableClientTransport{
				Endpoint:   endpoint,
				HTTPClient: client,
				MaxRetries: retries,
			}, nil
		}
	}

	if b.sseURL != "" {
		endpoint := b.sseURL
		client := withHeaders(b.httpClient, b.headers)
		return func() (sdkmcp.Transport, error) {
			return &sdkmcp.SSEClientTransport{
				Endpoint:   endpoint,
				HTTPClient: client,
			}, nil
		}
	}

	if b.ioSpec != nil {
		spec := b.ioSpec
		return func() (sdkmcp.Transport, error) {
			return &sdkmcp.IOTransport{Reader: spec.reader, Writer: spec.writer}, nil
		}
	}

	return nil
}

func (b *Builder) ensureCommandSpec() *commandSpec {
	if b.command == nil {
		b.command = &commandSpec{env: make(map[string]string)}
	}
	return b.command
}

func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, env := range base {
		key, _, ok := splitEnv(env)
		if ok {
			if _, found := overrides[key]; found {
				continue
			}
		}
		out = append(out, env)
	}
	for key, value := range overrides {
		if key == "" {
			continue
		}
		out = append(out, key+"="+value)
	}
	return out
}

func splitEnv(value string) (string, string, bool) {
	for i := 0; i < len(value); i++ {
		if value[i] == '=' {
			return value[:i], value[i+1:], true
		}
	}
	return value, "", false
}
