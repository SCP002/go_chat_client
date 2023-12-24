package chat

import (
	"time"

	"go_chat_client/config"
	"go_chat_client/connection"
	"go_chat_client/ui"
	stdinUtil "go_chat_client/util/stdin"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// loginReq represents login request to server.
type loginReq struct {
	Type     float64 `json:"type"`
	Nickname string  `json:"nickname"`
}

// loginResp represents login response from server.
type loginResp struct {
	Type   float64 `json:"type"`
	Token  string  `json:"token"`
	Status float64 `json:"status"`
}

// postMsgReq respresents post message request to server.
type postMsgReq struct {
	Type  float64 `json:"type"`
	Token string  `json:"token"`
	Msg   string  `json:"msg"`
}

// postMsgResp represents post message response from server.
type postMsgResp struct {
	Type   float64 `json:"type"`
	Status float64 `json:"status"`
}

// chatMsgToClient represents message to print in client's chat box.
type chatMsgToClient struct {
	Type     float64 `json:"type"`
	Nickname string  `json:"nickname"`
	Msg      string  `json:"msg"`
	IsSystem bool    `json:"isSystem"`
}

// onlineUsersReq represents request for list of online users to send to server.
type onlineUsersReq struct {
	Type  float64 `json:"type"`
	Token string  `json:"token"`
}

// onlineUsers represent list of online users received from server.
type onlineUsers struct {
	Type   float64  `json:"type"`
	Status float64  `json:"status"`
	Users  []string `json:"users"`
}

// used to distinguish between types of various JSON requests and responses.
const (
	typeLoginReq float64 = iota + 1
	typeLoginResp
	typePostMessageReq
	typePostMessageResp
	typeChatMessageToClient
	typeOnlineUsersReq
	typeOnlineUsers
)

// represents various statuses to receive in responses from server.
const (
	statusOk float64 = iota + 1
	statusInvalidToken
	statusNameAlreadyTaken
	statusNameIsEmpty
	statusNameIsTooLong
	statusMessageIsEmpty
	statusMessageIsTooLong
)

// Handler represents communication logic handler. It handles responses and sends requests.
type Handler struct {
	ChatUI  ui.Chat
	log     *logrus.Logger
	cfg     *config.Config
	conn    *connection.Handler
	tokenCh chan string
	token   string
}

// NewHandler returns new chat handler.
func NewHandler(log *logrus.Logger, cfg *config.Config, conn *connection.Handler) Handler {
	return Handler{log: log, cfg: cfg, conn: conn, tokenCh: make(chan string)}
}

// HandleOnDisconnect performs actions to do when connection to server is lost.
func (h *Handler) HandleOnDisconnect() {
	h.conn.AddOnDisconnectListener(func(err error) {
		h.log.Error(errors.Wrap(err, "Lost connection to server"), " Retrying in 5 seconds.")
		if !lo.IsEmpty(&h.ChatUI) {
			h.ChatUI.OnlineUsersCh <- []string{}
		}
		time.Sleep(time.Second * 5)
		h.conn.Connect()
		if err := h.login(); err != nil {
			h.log.Error(err)
		}
		go func() {
			h.token = <-h.tokenCh
		}()
	})
}

// HandleLoginResponse performs actions to do when server responds with login status and access token.
func (h *Handler) HandleLoginResponse() {
	h.conn.AddOnRespListener(func(resp map[string]any) {
		if resp["type"] != typeLoginResp {
			return
		}
		var r loginResp
		err := mapstructure.Decode(resp, &r)
		if err != nil {
			h.log.Error(errors.Wrap(err, "Decode login status response"))
			return
		}
		switch r.Status {
		case statusOk:
			h.log.Info("Login successful")
			h.tokenCh <- r.Token
		case statusNameAlreadyTaken:
			h.log.Warn("Name is already taken")
			h.cfg.Nickname = stdinUtil.AskNickname(h.log)
			if err := h.login(); err != nil {
				h.log.Error(err)
			}
		default:
			h.log.Error("Login failed, status: ", r.Status)
		}
	})
}

// LoginAndWaitForToken sends login request and blocks until access token is received back.
func (h *Handler) LoginAndWaitForToken() {
	if err := h.login(); err != nil {
		h.log.Error(err)
	}
	h.token = <-h.tokenCh
}

// PostMessage sends post message request to server.
func (h *Handler) PostMessage(msg string) {
	err := h.conn.WriteJSON(postMsgReq{Type: typePostMessageReq, Token: h.token, Msg: msg})
	if err != nil {
		h.log.Error(errors.Wrap(err, "Send post message request"))
	}
}

// PostMessage sends online useres list request to server.
func (h *Handler) RequestOnlineUsers() {
	if err := h.conn.WriteJSON(onlineUsersReq{Type: typeOnlineUsersReq, Token: h.token}); err != nil {
		h.log.Error(errors.Wrap(err, "Send online users request"))
	}
}

// HandleChatMsgToClient performs actions to do when server sends chat message to client.
func (h *Handler) HandleChatMsgToClient() {
	h.conn.AddOnRespListener(func(resp map[string]any) {
		if resp["type"] != typeChatMessageToClient {
			return
		}
		var r chatMsgToClient
		err := mapstructure.Decode(resp, &r)
		if err != nil {
			h.log.Error(errors.Wrap(err, "Decode chat message to client"))
			return
		}
		if err := h.ChatUI.PrintToChatBox(r.Nickname, r.Msg, r.IsSystem); err != nil {
			h.log.Error(err)
		}
	})
}

// HandlePostMessageResponse performs actions to do when server responds with status if message was posted.
func (h *Handler) HandlePostMessageResponse() {
	h.conn.AddOnRespListener(func(resp map[string]any) {
		if resp["type"] != typePostMessageResp {
			return
		}
		var r postMsgResp
		err := mapstructure.Decode(resp, &r)
		if err != nil {
			h.log.Error(errors.Wrap(err, "Decode post message status response"))
			return
		}
		if r.Status != statusOk {
			h.log.Error("Post message failed, status: ", r.Status)
		}
	})
}

// HandleOnlineUsers performs actions to do when server sends online users list to client.
func (h *Handler) HandleOnlineUsers() {
	h.conn.AddOnRespListener(func(resp map[string]any) {
		if resp["type"] != typeOnlineUsers {
			return
		}
		var r onlineUsers
		err := mapstructure.Decode(resp, &r)
		if err != nil {
			h.log.Error(errors.Wrap(err, "Decode online users response"))
			return
		}
		if r.Status == statusOk {
			h.ChatUI.OnlineUsersCh <- r.Users
		} else {
			h.log.Error("Get online users failed, status: ", r.Status)
		}
	})
}

// login sends login request to server.
func (h *Handler) login() error {
	err := h.conn.WriteJSON(loginReq{Type: typeLoginReq, Nickname: h.cfg.Nickname})
	return errors.Wrap(err, "Send login request")
}
