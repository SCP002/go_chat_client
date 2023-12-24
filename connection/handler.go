package connection

import (
	"net"
	"net/url"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// Handler represents connection handler. It wraps websocket connection with convenient methods.
type Handler struct {
	log          *logrus.Logger
	conn         *websocket.Conn
	url          url.URL
	onResponse   []func(map[string]any)
	onDisconnect []func(error)
}

// NewHandler returns new connection handler. <addr> should be specified in form of 'host:port'. If <tls> is true,
// establish secure connection to server.
func NewHandler(log *logrus.Logger, tls bool, addr string) *Handler {
	u := url.URL{Scheme: lo.Ternary(tls, "wss", "ws"), Host: addr, Path: "/chat"}
	return &Handler{log: log, url: u}
}

// Connect connects to server, blocks until connection if successfull and sets Handler.conn field with connection if so.
func (h *Handler) Connect() {
	for {
		conn, _, err := websocket.DefaultDialer.Dial(h.url.String(), nil)
		if err == nil {
			h.conn = conn
			h.log.Info("Connected to ", h.url.Host)
			return
		} else {
			h.log.Error(errors.Wrap(err, "Connect to server"), " Retrying in 5 seconds.")
			time.Sleep(time.Second * 5)
		}
	}
}

// AddOnDisconnectListener registers function <l> to be run when connection to server is lost.
func (h *Handler) AddOnDisconnectListener(l func(error)) {
	h.onDisconnect = append(h.onDisconnect, l)
}

// CloseConn sends close message to server and closes underlying network connection.
func (h *Handler) CloseConn() {
	err := h.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		h.log.Error(errors.Wrap(err, "Write close connection message"))
	}
	if err = h.conn.Close(); err != nil {
		h.log.Error(errors.Wrap(err, "Close connection"))
	}
}

// AddOnRespListener registers function <l> to be run when client receives a message from server.
func (h *Handler) AddOnRespListener(l func(map[string]any)) {
	h.onResponse = append(h.onResponse, l)
}

// Listen listens for incoming messages, blocking current goroutine until unknown read error occurs. It runs
// on disconnect and on response listeners.
func (h *Handler) Listen() error {
	for {
		var resp map[string]any
		err := h.conn.ReadJSON(&resp)
		var closeErr *websocket.CloseError
		var netErr net.Error
		if errors.As(err, &closeErr) || errors.As(err, &netErr) {
			for _, listener := range h.onDisconnect {
				listener(err)
			}
			continue
		} else if err != nil {
			return errors.Wrap(err, "Read JSON from connection")
		}
		for _, listener := range h.onResponse {
			listener(resp)
		}
	}
}

// Sends JSON encoding of <req> to server.
func (h *Handler) WriteJSON(req any) error {
	return h.conn.WriteJSON(req)
}
