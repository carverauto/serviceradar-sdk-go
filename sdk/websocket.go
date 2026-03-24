package sdk

import (
	"context"
	"errors"
	"time"
)

var errWebSocketConnNotInitialized = errors.New("websocket connection not initialized")

// WebSocketConn wraps a host websocket connection handle.
type WebSocketConn struct {
	handle uint32
}

// WebSocketDial opens a websocket connection via the host proxy.
func WebSocketDial(rawURL string, timeout time.Duration) (*WebSocketConn, error) {
	return WebSocketDialContext(context.Background(), rawURL, timeout)
}

// WebSocketDialContext opens a websocket connection via the host proxy with a context.
func WebSocketDialContext(ctx context.Context, rawURL string, timeout time.Duration) (*WebSocketConn, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	data := []byte(rawURL)
	res := hostWebSocketConnect(ptrFromBytes(data), uint32(len(data)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return nil, hostErr(res, "websocket_connect")
	}

	return &WebSocketConn{handle: uint32(res)}, nil
}

// Send writes a websocket message via the host proxy.
func (c *WebSocketConn) Send(data []byte, timeout time.Duration) error {
	return c.SendContext(context.Background(), data, timeout)
}

// SendContext writes a websocket message via the host proxy with a context.
func (c *WebSocketConn) SendContext(ctx context.Context, data []byte, timeout time.Duration) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if c == nil || c.handle == 0 {
		return errWebSocketConnNotInitialized
	}
	if len(data) == 0 {
		return nil
	}

	res := hostWebSocketSend(c.handle, ptrFromBytes(data), uint32(len(data)), uint32(timeout.Milliseconds()))
	return hostErr(res, "websocket_send")
}

// Recv reads a websocket message via the host proxy into buf.
func (c *WebSocketConn) Recv(buf []byte, timeout time.Duration) (int, error) {
	return c.RecvContext(context.Background(), buf, timeout)
}

// RecvContext reads a websocket message via the host proxy into buf with a context.
func (c *WebSocketConn) RecvContext(ctx context.Context, buf []byte, timeout time.Duration) (int, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}

	if c == nil || c.handle == 0 {
		return 0, errWebSocketConnNotInitialized
	}
	if len(buf) == 0 {
		return 0, nil
	}

	res := hostWebSocketRecv(c.handle, ptrFromBytes(buf), uint32(len(buf)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return 0, hostErr(res, "websocket_recv")
	}

	return int(res), nil
}

// Close closes the websocket connection.
func (c *WebSocketConn) Close() error {
	if c == nil || c.handle == 0 {
		return nil
	}

	res := hostWebSocketClose(c.handle)
	c.handle = 0

	return hostErr(res, "websocket_close")
}
