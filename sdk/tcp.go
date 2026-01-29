package sdk

import (
	"errors"
	"time"
)

// TCPConn wraps a host TCP connection handle.
type TCPConn struct {
	handle uint32
}

// TCPDial opens a TCP connection via the host proxy.
func TCPDial(host string, port uint16, timeout time.Duration) (*TCPConn, error) {
	addr := []byte(host)
	res := hostTCPConnect(ptrFromBytes(addr), uint32(len(addr)), uint32(port), uint32(timeout.Milliseconds()))
	if res < 0 {
		return nil, hostErr(res, "tcp_connect")
	}
	return &TCPConn{handle: uint32(res)}, nil
}

// Read reads from the host connection into buf.
func (c *TCPConn) Read(buf []byte, timeout time.Duration) (int, error) {
	if c == nil || c.handle == 0 {
		return 0, errors.New("tcp connection not initialized")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	res := hostTCPRead(c.handle, ptrFromBytes(buf), uint32(len(buf)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return 0, hostErr(res, "tcp_read")
	}
	return int(res), nil
}

// Write writes data to the host connection.
func (c *TCPConn) Write(data []byte, timeout time.Duration) (int, error) {
	if c == nil || c.handle == 0 {
		return 0, errors.New("tcp connection not initialized")
	}
	if len(data) == 0 {
		return 0, nil
	}
	res := hostTCPWrite(c.handle, ptrFromBytes(data), uint32(len(data)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return 0, hostErr(res, "tcp_write")
	}
	return int(res), nil
}

// Close closes the host connection.
func (c *TCPConn) Close() error {
	if c == nil || c.handle == 0 {
		return nil
	}
	res := hostTCPClose(c.handle)
	c.handle = 0
	return hostErr(res, "tcp_close")
}
