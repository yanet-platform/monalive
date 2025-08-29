// Package yanet provides a client for interacting with the YANET control plane
// via gRPC over a Unix domain socket. This package allows users to invoke
// methods on the YANET balancer service using a custom RPC channel.
package yanet

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	yanetpb "github.com/yanet-platform/monalive/gen/yanet"
)

const defaultYANETControlPlaneSockPath = "/run/yanet/protocontrolplane.sock"

// Type alias to make embedded field unexportable.
type balancerServiceClient = yanetpb.BalancerServiceClient

// Client provides methods for interacting with the YANET balancer service. It
// connects to the YANET control plane via a Unix domain socket.
type Client struct {
	yanetControlPlaneSockPath string
	balancerServiceClient
}

// wrapError wraps an error with the method name.
func wrapError(method string, err error) error {
	if err != nil {
		return fmt.Errorf("yanetproto.%s: %w", method, err)
	}
	return nil
}

// methods holds the list of available methods from the BalancerService.
var methods []string

// init initializes the list of methods available in the BalancerService.
func init() {
	// Use a dummy service to reflect on the available methods.
	var dummyService *yanetpb.UnimplementedBalancerServiceServer
	serviceType := reflect.TypeOf(dummyService)

	// Populate the methods slice with method names.
	methods = make([]string, 0, serviceType.NumMethod())
	for i := 0; i < serviceType.NumMethod(); i++ {
		methods = append(methods, serviceType.Method(i).Name)
	}
}

// NewClient creates a new Client with optional configuration. The client
// connects to the default YANET control plane socket unless overridden by the
// provided ClientOption.
func NewClient(opts ...ClientOption) (*Client, error) {
	client := &Client{
		yanetControlPlaneSockPath: defaultYANETControlPlaneSockPath,
	}

	// Apply all the provided options to the client.
	for _, opt := range opts {
		opt(client)
	}

	channel, err := newRPCChannel(client.yanetControlPlaneSockPath)
	if err != nil {
		return nil, err
	}
	client.balancerServiceClient = yanetpb.NewBalancerServiceClient(channel)

	return client, nil
}

// ClientOption represents an option for configuring the Client.
type ClientOption func(*Client)

// WithControlPlaneSockPath sets a custom Unix domain socket path for the
// Client.
func WithControlPlaneSockPath(path string) ClientOption {
	return func(c *Client) {
		c.yanetControlPlaneSockPath = path
	}
}

// rpcChannel implements grpc.ClientConnInterface and manages the RPC
// connections for each method.
//
// In fact rpcChannel stores set of connections to YANET control plane. Each
// connection refers to one of the RPC calls provided by YANET balancer service.
//
// This structure is lock-free because conn map does not change after creation
// of this object.
type rpcChannel struct {
	sockPath string              // path to YANET control plane socket
	conn     map[string]*rpcConn // maps method name to rpcConn
}

// newRPCChannel initializes a new rpcChannel for the given Unix domain socket
// path.
func newRPCChannel(sockPath string) (*rpcChannel, error) {
	conn := make(map[string]*rpcConn, len(methods))

	// Create an RPC connection for each method.
	for _, method := range methods {
		rpcConn, err := newRPCConn(sockPath, method)
		if err != nil {
			return nil, err
		}
		conn[method] = rpcConn
	}

	return &rpcChannel{
		sockPath: sockPath,
		conn:     conn,
	}, nil
}

// Invoke performs an RPC invocation for a specified method.
//
// Firstly this function extracts method name from provided RPC method.
// Then its find the appropriate RPC connection based on the method name.
// Finally it invokes the RPC method with the provided input and output
// messages.
func (m *rpcChannel) Invoke(ctx context.Context, method string, in interface{}, out interface{}, opts ...grpc.CallOption) error {
	// Extract service name and method name from method.
	// Method example: /common.icp_proto.BalancerService/Real
	// 	Service name: BalancerService
	// 	Method name: Real
	parts := strings.FieldsFunc(method, func(r rune) bool {
		return r == '/' || r == '.'
	})
	if len(parts) < 2 {
		return fmt.Errorf("invalid method: %s", method)
	}
	serviceName, methodName := parts[len(parts)-2], parts[len(parts)-1]

	// Find the appropriate RPC connection based on the method name.
	conn, exists := m.conn[methodName]
	if !exists {
		return fmt.Errorf("method %q not found", methodName)
	}

	// Prepare the RPC metadata for the invocation.
	meta := yanetpb.RpcMeta{
		ServiceName_: &yanetpb.RpcMeta_ServiceName{ServiceName: serviceName},
		MethodName_:  &yanetpb.RpcMeta_MethodName{MethodName: methodName},
	}

	// Invoke the RPC method with the provided input and output messages.
	return conn.Invoke(&meta, in.(proto.Message), out.(proto.Message))
}

// NewStream creates a new stream for the given method. Not implemented in this
// version.
func (m *rpcChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
	defer func() {
		err = wrapError(method, err)
	}()

	// Stream methods are currently not supported.
	return nil, fmt.Errorf("Unimplemented")
}

// rpcConn represents a single RPC connection for a method.
type rpcConn struct {
	conn   net.Conn   // Unix domain socket connection
	method string     // RPC method name
	closed bool       // whether the connection is closed
	mu     sync.Mutex // concurrent calls lead to messages mix up, so use mutex
}

// newRPCConn initializes a new rpcConn for the given method.
func newRPCConn(sockPath string, method string) (*rpcConn, error) {
	// Establish a connection to the Unix socket.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	return &rpcConn{
		conn:   conn,
		method: method,
		closed: false,
	}, nil
}

// Invoke performs an RPC invocation by sending and receiving protobuf messages.
func (m *rpcConn) Invoke(meta *yanetpb.RpcMeta, in, out proto.Message) (err error) {
	defer func() {
		err = wrapError(m.method, err)
	}()
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reopen the connection if it is closed.
	if err := m.open(); err != nil {
		return err
	}

	// Send the RPC metadata.
	if err := m.send(meta); err != nil {
		return err
	}

	// Send the input protobuf message.
	if err := m.send(in); err != nil {
		return err
	}

	// Receive the output protobuf message.
	return m.recv(out)
}

// send serializes and sends a protobuf message over the connection.
func (m *rpcConn) send(message proto.Message) (err error) {
	// Serialize the protobuf message to bytes.
	messageBuf, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	// Prepare a buffer to store the size of the serialized message.
	sizeBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(sizeBuf, uint64(len(messageBuf)))

	// Write the size of the message to the Unix socket.
	if _, err := m.write(sizeBuf); err != nil {
		_ = m.close()
		return fmt.Errorf("write [%s]: %w", string(sizeBuf), err)
	}

	// Write the serialized message itself to the Unix socket.
	if _, err := m.write(messageBuf); err != nil {
		_ = m.close()
		return fmt.Errorf("write [%s]: %w", string(messageBuf), err)
	}

	return nil
}

// recv receives and deserializes a protobuf message from the connection.
func (m *rpcConn) recv(message proto.Message) (err error) {
	// Prepare a buffer to receive the size of the incoming message.
	sizeBuf := make([]byte, 8)

	// Read the message size in chunks until the buffer is filled.
	for n := 0; n < len(sizeBuf); {
		d, err := m.read(sizeBuf[n:])
		if err != nil {
			_ = m.close()
			return fmt.Errorf("read n: %w", err)
		}
		n += d
	}

	// If the message size is greater than zero, read the full message.
	if size := binary.LittleEndian.Uint64(sizeBuf); size > 0 {
		readBuf := make([]byte, size)
		// Read the message in chunks until the buffer is filled.
		for n := 0; n < len(readBuf); {
			d, err := m.read(readBuf[n:])
			if err != nil {
				_ = m.close()
				return fmt.Errorf("read: %w", err)
			}
			n += d
		}

		// Deserialize the message from bytes.
		if err := proto.Unmarshal(readBuf, message); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}

	return nil
}

// open reopens the connection if it is closed.
func (m *rpcConn) open() error {
	// If the connection is not closed, do nothing.
	if !m.closed {
		return nil
	}

	// Reopen the connection to the Unix socket.
	conn, err := net.Dial("unix", m.conn.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	m.conn = conn
	m.closed = false

	return nil
}

// write sends bytes over the connection.
func (m *rpcConn) write(b []byte) (int, error) {
	return m.conn.Write(b)
}

// read reads bytes from the connection.
func (m *rpcConn) read(b []byte) (int, error) {
	return m.conn.Read(b)
}

// close closes the connection.
func (m *rpcConn) close() error {
	// If the connection is already closed, do nothing.
	if m.closed {
		return nil
	}

	err := m.conn.Close()
	m.closed = true

	return err
}
