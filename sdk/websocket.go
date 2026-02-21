package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var errWebSocketNotInitialized = errors.New("websocket connection not initialized")

// WebSocketConn wraps a host WebSocket connection handle.
type WebSocketConn struct {
	handle uint32
}

type webSocketConnectPayload struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// WebSocketConnect opens a WebSocket connection via the host proxy.
func WebSocketConnect(url string, timeout time.Duration) (*WebSocketConn, error) {
	return WebSocketConnectContext(context.Background(), url, timeout)
}

// WebSocketConnectWithHeaders opens a WebSocket connection via the host proxy
// using optional request headers (for example Authorization).
func WebSocketConnectWithHeaders(url string, headers map[string]string, timeout time.Duration) (*WebSocketConn, error) {
	return WebSocketConnectContextWithHeaders(context.Background(), url, headers, timeout)
}

// WebSocketConnectContext opens a WebSocket connection via the host proxy with a context.
func WebSocketConnectContext(ctx context.Context, url string, timeout time.Duration) (*WebSocketConn, error) {
	return WebSocketConnectContextWithHeaders(ctx, url, nil, timeout)
}

// WebSocketConnectContextWithHeaders opens a WebSocket connection via the host
// proxy with optional request headers and context.
func WebSocketConnectContextWithHeaders(
	ctx context.Context,
	url string,
	headers map[string]string,
	timeout time.Duration,
) (*WebSocketConn, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	payload, err := encodeWebSocketConnectPayload(url, headers)
	if err != nil {
		return nil, err
	}
	res := hostWebSocketConnect(ptrFromBytes(payload), uint32(len(payload)), uint32(timeout.Milliseconds()))

	if res < 0 {
		return nil, hostErr(res, "websocket_connect")
	}
	return &WebSocketConn{handle: uint32(res)}, nil
}

func encodeWebSocketConnectPayload(url string, headers map[string]string) ([]byte, error) {
	if len(headers) == 0 {
		return []byte(url), nil
	}
	payload := webSocketConnectPayload{
		URL:     url,
		Headers: headers,
	}
	return json.Marshal(payload)
}

// Send sends data through the WebSocket connection.
func (ws *WebSocketConn) Send(data []byte, timeout time.Duration) error {
	return ws.SendContext(context.Background(), data, timeout)
}

// SendContext sends data through the WebSocket connection with a context.
func (ws *WebSocketConn) SendContext(ctx context.Context, data []byte, timeout time.Duration) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if ws == nil || ws.handle == 0 {
		return errWebSocketNotInitialized
	}
	if len(data) == 0 {
		return nil
	}

	res := hostWebSocketSend(ws.handle, ptrFromBytes(data), uint32(len(data)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return hostErr(res, "websocket_send")
	}

	return nil
}

// Recv receives data from the WebSocket connection into buf.
func (ws *WebSocketConn) Recv(buf []byte, timeout time.Duration) (int, error) {
	return ws.RecvContext(context.Background(), buf, timeout)
}

// RecvContext receives data from the WebSocket connection into buf with a context.
func (ws *WebSocketConn) RecvContext(ctx context.Context, buf []byte, timeout time.Duration) (int, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}

	if ws == nil || ws.handle == 0 {
		return 0, errWebSocketNotInitialized
	}
	if len(buf) == 0 {
		return 0, nil
	}

	res := hostWebSocketRecv(ws.handle, ptrFromBytes(buf), uint32(len(buf)), uint32(timeout.Milliseconds()))
	if res < 0 {
		return 0, hostErr(res, "websocket_recv")
	}

	return int(res), nil
}

// Close closes the WebSocket connection.
func (ws *WebSocketConn) Close() error {
	if ws == nil || ws.handle == 0 {
		return nil
	}
	res := hostWebSocketClose(ws.handle)

	ws.handle = 0

	return hostErr(res, "websocket_close")
}
