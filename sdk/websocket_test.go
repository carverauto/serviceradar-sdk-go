package sdk

import (
	"errors"
	"testing"
	"time"
)

func TestWebSocketConnRequiresHandle(t *testing.T) {
	t.Parallel()

	var conn *WebSocketConn
	if err := conn.Send([]byte("hello"), time.Second); !errors.Is(err, errWebSocketConnNotInitialized) {
		t.Fatalf("expected websocket handle error, got %v", err)
	}

	if _, err := conn.Recv(make([]byte, 16), time.Second); !errors.Is(err, errWebSocketConnNotInitialized) {
		t.Fatalf("expected websocket recv handle error, got %v", err)
	}
}
