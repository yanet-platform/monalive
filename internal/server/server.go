package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	monalivepb "monalive/gen/manager"
)

// Server is used to handle requests for various management operations with
// Monalive, such as checking the current configuration status and reloading it.
type Server struct {
	config     *Config
	manager    monalivepb.MonaliveManagerServer
	grpcServer *grpc.Server
	httpServer *http.Server
}

// New creates a new Server instance with the given configuration and gRPC
// manager. It initializes both gRPC and HTTP servers and registers necessary
// services.
func New(config *Config, manager monalivepb.MonaliveManagerServer) *Server {
	// Create a new gRPC server and register the MonaliveManagerServer
	// implementation.
	gRPCServer := grpc.NewServer()
	monalivepb.RegisterMonaliveManagerServer(gRPCServer, manager)

	// Register reflection service on gRPC server.
	reflection.Register(gRPCServer)

	// Initialize the HTTP server with the provided address.
	httpServer := &http.Server{
		Addr: config.HTTPAddr,
	}

	return &Server{
		config:     config,
		manager:    manager,
		grpcServer: gRPCServer,
		httpServer: httpServer,
	}
}

// Run starts both the gRPC and HTTP servers.
func (m *Server) Run(ctx context.Context) error {
	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		return m.runGRPCServer()
	})
	wg.Go(func() error {
		return m.runHTTPServer(ctx)
	})
	return wg.Wait()
}

// Stop gracefully stops both the gRPC and HTTP servers.
func (m *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m.grpcServer.Stop()
	_ = m.httpServer.Shutdown(ctx)
}

func (m *Server) runGRPCServer() error {
	listener, err := net.Listen("tcp", m.config.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	return m.grpcServer.Serve(listener)
}

func (m *Server) runHTTPServer(ctx context.Context) error {
	// Registers the MonaliveManagerHandlerServer to handle the HTTP requests.
	mux := runtime.NewServeMux(runtime.WithMarshalerOption("application/protobuf", &runtime.ProtoMarshaller{}))
	if err := monalivepb.RegisterMonaliveManagerHandlerServer(ctx, mux, m.manager); err != nil {
		return fmt.Errorf("failed to register handler: %w", err)
	}

	m.httpServer.Handler = mux

	return m.httpServer.ListenAndServe()
}
