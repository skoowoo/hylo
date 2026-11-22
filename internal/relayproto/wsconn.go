// Package relayproto holds shared wire constants and WebSocket stream adapters
// used by both the public Relay and the home Connector.
package relayproto

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ControlPath = "/relay/v1/control"
	DataPath    = "/relay/v1/data"

	HeaderServer  = "X-Hylo-Relay-Server"
	HeaderAccess  = "X-Hylo-Relay-Access"
	HeaderSession = "X-Hylo-Relay-Session"
	HeaderJoinKey = "X-Hylo-Relay-Join-Key"
	HeaderKey     = "X-Hylo-Relay-Key"
)

// WSConn exposes WebSocket binary messages as a continuous net.Conn byte stream.
// Message boundaries are invisible to callers.
type WSConn struct {
	c      *websocket.Conn
	local  wsAddr
	remote wsAddr
	r      io.Reader
	rmu    sync.Mutex
	wmu    sync.Mutex
}

func NewWSConn(c *websocket.Conn, local, remote string) *WSConn {
	return &WSConn{c: c, local: wsAddr(local), remote: wsAddr(remote)}
}

func (w *WSConn) Read(p []byte) (int, error) {
	w.rmu.Lock()
	defer w.rmu.Unlock()
	for {
		if w.r != nil {
			n, err := w.r.Read(p)
			if err == io.EOF {
				w.r = nil
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}
		kind, r, err := w.c.NextReader()
		if err != nil {
			return 0, err
		}
		if kind == websocket.BinaryMessage {
			w.r = r
		}
	}
}

func (w *WSConn) Write(p []byte) (int, error) {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	x, err := w.c.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	n, writeErr := x.Write(p)
	closeErr := x.Close()
	if writeErr != nil {
		return n, writeErr
	}
	return n, closeErr
}

func (w *WSConn) Close() error                       { return w.c.Close() }
func (w *WSConn) LocalAddr() net.Addr                { return w.local }
func (w *WSConn) RemoteAddr() net.Addr               { return w.remote }
func (w *WSConn) SetReadDeadline(t time.Time) error  { return w.c.SetReadDeadline(t) }
func (w *WSConn) SetWriteDeadline(t time.Time) error { return w.c.SetWriteDeadline(t) }
func (w *WSConn) SetDeadline(t time.Time) error {
	return errors.Join(w.c.SetReadDeadline(t), w.c.SetWriteDeadline(t))
}

type wsAddr string

func (a wsAddr) Network() string { return "websocket" }
func (a wsAddr) String() string  { return string(a) }
