// Package bird provides a client for communicating with a BIRD routing daemon
// using Unix domain sockets.
package bird

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"syscall"
	"unsafe"
)

// ErrEmptySock is returned when attempting to send a message using an
// uninitialized socket.
var ErrEmptySock = errors.New("empty sock")

const (
	defaultBatchSize = 4096                           // defines the default size for message batching
	messageSize      = uint(unsafe.Sizeof(Message{})) // represents the size of a Message struct in bytes
)

// Client is a client for communicating with the BIRD routing daemon. It manages
// Unix domain sockets for sending and receiving messages.
type Client struct {
	sock       *net.UnixConn // used for communication
	listenAddr *net.UnixAddr // the address to listen for incoming messages
	writeAddr  *net.UnixAddr // the address to send outgoing messages

	batchSize uint // the size of the batch when sending messages in bulk
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithBatchSize is a ClientOption that sets the batch size for the Client.
func WithBatchSize(size uint) ClientOption {
	return func(c *Client) {
		c.batchSize = size
	}
}

// NewClient creates a new Client with the specified socket directory and name
// prefix. The client listens and sends messages using Unix domain sockets.
// Additional options can be provided using the ClientOption functions.
func NewClient(sockDir, sockNamePrefix string, opts ...ClientOption) (*Client, error) {
	client := &Client{
		batchSize: defaultBatchSize,
	}

	// Apply all the provided options to the client.
	for _, opt := range opts {
		opt(client)
	}

	// Set up the listen socket address.
	listenSockName := sockNamePrefix + "_b2m"
	listenAddr := &net.UnixAddr{
		Name: path.Join(sockDir, listenSockName),
		Net:  "unixgram",
	}

	// Remove the previous socket file if it exists.
	err := os.Remove(listenAddr.Name)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove %q: %w", listenAddr.Name, err)
	}

	// Create a new Unixgram socket and bind it to the listen address.
	client.sock, err = net.ListenUnixgram("unixgram", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to %q: %w", listenAddr.Name, err)
	}

	// Get the current file permissions of the socket.
	var stat syscall.Stat_t
	if err := syscall.Stat(listenAddr.Name, &stat); err != nil {
		return nil, fmt.Errorf("failed to get permissions of %q: %w", listenAddr.Name, err)
	}

	// Add write permissions for others and group.
	if err := os.Chmod(listenAddr.Name, fs.FileMode(stat.Mode)|syscall.S_IWOTH|syscall.S_IWGRP); err != nil {
		return nil, fmt.Errorf("failed to add write permission to %q: %w", listenAddr.Name, err)
	}

	client.listenAddr = listenAddr

	// Set up the write socket address.
	writeSockName := sockNamePrefix + "_m2b"
	client.writeAddr = &net.UnixAddr{
		Name: path.Join(sockDir, writeSockName),
		Net:  "unixgram",
	}

	return client, nil
}

// ListenRequest listens for an incoming request on the client's socket. This is
// a blocking call that waits for a message to be received.
func (m *Client) ListenRequest() error {
	// This array is only used to ensure that the Read function does not return
	// immediately. The received data is not used due to the Monalive-BIRD
	// protocol's implementation details.
	var dummyBuf [8]byte
	if _, err := m.sock.Read(dummyBuf[:]); err != nil {
		return fmt.Errorf("%s: %w", m.listenAddr.Name, err)
	}

	return nil
}

// Send sends a single [Message] to the BIRD daemon. The message is serialized
// using BigEndian byte order before sending.
func (m *Client) Send(msg Message) error {
	// Create a buffer to hold the serialized message.
	buf := bytes.NewBuffer(make([]byte, 0, messageSize))
	if err := binary.Write(buf, binary.BigEndian, msg); err != nil {
		return err
	}

	return m.send(buf.Bytes())
}

// SendBatch sends a batch of [Message] to the BIRD daemon. The messages are sent
// in batches according to the client's batch size.
func (m *Client) SendBatch(msgs ...Message) error {
	batchSize := min(m.batchSize, uint(len(msgs)))
	bufSize := messageSize * batchSize
	msgBuf := bytes.NewBuffer(make([]byte, 0, bufSize))

	// Serialize and send messages in batches.
	for _, msg := range msgs {
		if err := binary.Write(msgBuf, binary.BigEndian, msg); err != nil {
			return fmt.Errorf("failed to write message to buffer: %w", err)
		}

		if uint(msgBuf.Len())+messageSize > bufSize {
			if err := m.send(msgBuf.Bytes()); err != nil {
				return err
			}
			msgBuf.Reset()
		}
	}
	return m.send(msgBuf.Bytes())
}

// send sends the given byte buffer to the BIRD daemon using the client's
// socket.
func (m *Client) send(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}

	if m.sock == nil {
		return fmt.Errorf("failed to send message to bird: %w", ErrEmptySock)
	}

	if _, err := m.sock.WriteToUnix(buf, m.writeAddr); err != nil {
		return fmt.Errorf("failed to send message to bird: %w", err)
	}

	return nil
}

func (m *Client) Shutdown() {
	_ = m.sock.Close()
}
