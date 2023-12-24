package main

import (
	"fmt"
	"os"

	"go_chat_client/chat"
	"go_chat_client/cli"
	"go_chat_client/config"
	"go_chat_client/connection"
	"go_chat_client/logger"
	"go_chat_client/ui"
	stdinUtil "go_chat_client/util/stdin"

	goFlags "github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logger.New(logrus.FatalLevel, os.Stderr)

	flags, err := cli.Parse()
	if flags.Version {
		fmt.Println("v0.1.0")
		os.Exit(0)
	}
	if cli.IsErrOfType(err, goFlags.ErrHelp) {
		// Help message will be prined by go-flags
		os.Exit(0)
	}
	if err != nil {
		log.Fatal(err)
	}

	log.SetLevel(flags.LogLevel)

	cfg, err := config.Read()
	if err != nil {
		log.Debug(err)
	}

	if cfg.ServerAddress == "" {
		cfg.ServerAddress = stdinUtil.AskServerAddress(log)
	}
	if cfg.TLSMode == nil {
		cfg.TLSMode = stdinUtil.AskTLSMode(log)
	}

	connHandler := connection.NewHandler(log, *cfg.TLSMode, cfg.ServerAddress)
	connHandler.Connect()

	defer connHandler.CloseConn()

	if cfg.Nickname == "" {
		cfg.Nickname = stdinUtil.AskNickname(log)
	}

	go func() {
		if err := connHandler.Listen(); err != nil {
			log.Fatal(err)
		}
	}()

	chatHandler := chat.NewHandler(log, cfg, connHandler)

	chatHandler.HandleOnDisconnect()
	chatHandler.HandleLoginResponse()
	chatHandler.LoginAndWaitForToken()

	chatUI, err := ui.NewChat(log)
	if err != nil {
		log.Fatal(err)
	}
	chatHandler.ChatUI = chatUI
	go func() {
		err := chatUI.Draw()
		if err != nil {
			log.Fatal(err)
		}
	}()
	go chatUI.UpdateOnlineBox()

	chatBoxView := chatUI.WaitForView(ui.ChatBoxName)
	log.SetOutput(chatBoxView)
	log.AddHook(logger.NewChatUIHook(chatUI.Gui))

	chatUI.AddOnMsgSendListener(chatHandler.PostMessage)
	chatUI.AddOnOnlineBoxOpenListener(chatHandler.RequestOnlineUsers)

	chatHandler.HandleChatMsgToClient()
	chatHandler.HandlePostMessageResponse()
	chatHandler.HandleOnlineUsers()

	if err = config.Write(cfg); err != nil {
		log.Error(err)
	}

	select {}
}
