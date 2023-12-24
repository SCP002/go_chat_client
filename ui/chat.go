package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// represents names for various views.
const (
	ChatBoxName    = "chat_box"
	inputFieldName = "input_field"
	onlineBoxName  = "online_box"
)

// Chat represents UI for chat window.
type Chat struct {
	Gui             *gocui.Gui
	OnlineUsersCh   chan []string
	log             *logrus.Logger
	visibleViews    []string
	currentViewIdx  int
	onMsgSend       []func(string)
	onOnlineBoxOpen []func()
}

// NewChat returns new UI for chat window and starts it's initializaton.
func NewChat(log *logrus.Logger) (Chat, error) {
	gui, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return Chat{}, errors.Wrap(err, "Create GUI")
	}

	gui.Highlight = true
	gui.Cursor = true
	gui.SelFgColor = gocui.ColorGreen

	return Chat{Gui: gui, OnlineUsersCh: make(chan []string), log: log}, nil
}

// WaitForView returns view with the specified <name> as soon as it becomes available.
func (c *Chat) WaitForView(name string) *gocui.View {
	viewCh := make(chan *gocui.View)
	c.Gui.Update(func(g *gocui.Gui) error {
		for {
			view, err := g.View(name)
			if err != nil {
				continue
			}
			viewCh <- view
			return nil
		}
	})
	return <-viewCh
}

// AddOnMsgSendListener registers function <l> to be run when message from input field is sent.
func (c *Chat) AddOnMsgSendListener(l func(string)) {
	c.onMsgSend = append(c.onMsgSend, l)
}

// AddOnOnlineBoxOpenListener registers function <l> to be run when online users box is open.
func (c *Chat) AddOnOnlineBoxOpenListener(l func()) {
	c.onOnlineBoxOpen = append(c.onOnlineBoxOpen, l)
}

// Draw sets layout managers, sets keybindings and runs main UI loop, finishing initialization. It blocks until Ctrl+C
// is pressed or unknown error occurs.
func (c *Chat) Draw() error {
	c.Gui.SetManager(gocui.ManagerFunc(c.chatBoxLayout), gocui.ManagerFunc(c.inputFieldLayout))

	if err := c.Gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding("", gocui.KeyTab, gocui.ModNone, c.nextView); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding("", gocui.KeyF2, gocui.ModNone, c.toggleOnlineBox); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding(inputFieldName, gocui.KeyEnter, gocui.ModNone, c.sendMessage); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	// Insert new line on F3.
	// Why not Shift+Enter? - This library only supports Alt modifier.
	// Why not Alt+Enter? - On Windows, Alt+Enter toggles console window fullscreen mode.
	if err := c.Gui.SetKeybinding(inputFieldName, gocui.KeyF3, gocui.ModNone, insertNewline); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding(ChatBoxName, gocui.KeyArrowUp, gocui.ModNone, scrollUp); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding(ChatBoxName, gocui.KeyArrowDown, gocui.ModNone, scrollDown); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding(onlineBoxName, gocui.KeyArrowUp, gocui.ModNone, scrollUp); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}
	if err := c.Gui.SetKeybinding(onlineBoxName, gocui.KeyArrowDown, gocui.ModNone, scrollDown); err != nil {
		return errors.Wrap(err, "Set keybinding")
	}

	if err := c.Gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		return errors.Wrap(err, "Run main UI loop")
	}

	return nil
}

// UpdateOnlineBox redraw online users box as soon as list of users is received from the respective channel.
// It blocks current goroutine forever.
func (c *Chat) UpdateOnlineBox() {
	for {
		onlineUsers := <-c.OnlineUsersCh

		c.Gui.Update(func(g *gocui.Gui) error {
			onlineBox, err := g.View(onlineBoxName)
			if err != nil {
				return nil
			}

			onlineBox.Clear()
			onlineBox.Title = fmt.Sprintf("%v online", len(onlineUsers))

			slices.Sort(onlineUsers)
			_, err = fmt.Fprint(onlineBox, strings.Join(onlineUsers, "\n"))
			if err != nil {
				c.log.Error(errors.Wrap(err, "Print online users"))
			}

			return nil
		})
	}
}

// PrintToChatBox prints <msg> to chat chat box view, prefixed with current time and <nickname>. If <isSystem> is true,
// <nickname> is replaced with "SYSTEM" and printed with another color.
func (c *Chat) PrintToChatBox(nickname string, msg string, isSystem bool) error {
	chatBox, err := c.Gui.View(ChatBoxName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Get view %v", ChatBoxName))
	}
	time := color.GreenString("%v", time.Now().Format("15:04:05"))
	if isSystem {
		nickname = color.CyanString("%v", "SYSTEM")
	} else {
		nickname = color.YellowString("%v", nickname)
	}

	_, err = fmt.Fprintln(chatBox, time, nickname, msg)
	if err != nil {
		return errors.Wrap(err, "Print message to chat box")
	}

	c.Gui.Update(func(g *gocui.Gui) error {
		return nil
	})

	return nil
}

// chatBoxLayout is a GUI manager function for chat box.
func (c *Chat) chatBoxLayout(gui *gocui.Gui) error {
	maxX, maxY := gui.Size()

	chatBox, err := gui.SetView(ChatBoxName, 0, 0, maxX-1, maxY-8)
	if !errors.Is(err, gocui.ErrUnknownView) {
		return errors.Wrap(err, fmt.Sprintf("Create view for %v", ChatBoxName))
	}
	c.visibleViews = append(c.visibleViews, ChatBoxName)
	chatBox.Title = "Chat"
	chatBox.Wrap = true
	chatBox.Autoscroll = true

	return nil
}

// inputFieldLayout is a GUI manager function for input field.
func (c *Chat) inputFieldLayout(gui *gocui.Gui) error {
	maxX, maxY := gui.Size()

	inputField, err := gui.SetView(inputFieldName, 0, maxY-7, maxX-1, maxY-1)
	if !errors.Is(err, gocui.ErrUnknownView) {
		return errors.Wrap(err, fmt.Sprintf("Create view for %v", inputFieldName))
	}
	c.visibleViews = append(c.visibleViews, inputFieldName)
	inputField.Title = "Input"
	inputField.Editable = true
	inputField.Wrap = true
	inputField.Editor = gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
		maxSymbols := 2000
		if len(v.Buffer()) <= maxSymbols {
			gocui.DefaultEditor.Edit(v, key, ch, mod)
			return
		}
		switch {
		case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
			v.EditDelete(true)
		case key == gocui.KeyDelete:
			v.EditDelete(false)
		case key == gocui.KeyArrowDown:
			v.MoveCursor(0, 1, false)
		case key == gocui.KeyArrowUp:
			v.MoveCursor(0, -1, false)
		case key == gocui.KeyArrowLeft:
			v.MoveCursor(-1, 0, false)
		case key == gocui.KeyArrowRight:
			v.MoveCursor(1, 0, false)
		default:
			c.log.Warnf("Message is longer than %v symbols", maxSymbols)
		}
	})

	if _, err = gui.SetCurrentView(inputFieldName); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Focus view %v", inputFieldName))
	}

	return nil
}

// sendMessage runs listeners passing trimmed input field buffer to them, clears input filed and sets cursor to initial
// position.
func (c *Chat) sendMessage(gui *gocui.Gui, view *gocui.View) error {
	inputField, err := gui.View(inputFieldName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Get view %v", inputFieldName))
	}

	for _, listener := range c.onMsgSend {
		listener(strings.TrimSpace(inputField.Buffer()))
	}

	inputField.Clear()
	if err = inputField.SetCursor(0, 0); err != nil {
		return errors.Wrap(err, "Reset cursor after message was sent")
	}

	return nil
}

// nextView cycling between views, focusing next visible one on each call.
func (c *Chat) nextView(gui *gocui.Gui, view *gocui.View) error {
	nextViewIdx := (c.currentViewIdx + 1) % len(c.visibleViews)
	nextView := c.visibleViews[nextViewIdx]

	if _, err := gui.SetCurrentView(nextView); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Focus view %v", nextView))
	}

	gui.Cursor = lo.Ternary(view.Name() == ChatBoxName, true, false)

	c.currentViewIdx = nextViewIdx

	return nil
}

// toggleOnlineBox opens online users box if it's closed and closes it if it's open.
func (c *Chat) toggleOnlineBox(gui *gocui.Gui, view *gocui.View) error {
	_, err := gui.View(onlineBoxName)

	if errors.Is(err, gocui.ErrUnknownView) {
		maxX, maxY := gui.Size()

		onlineBox, err := gui.SetView(onlineBoxName, maxX-20, 0, maxX-1, maxY-8)
		if !errors.Is(err, gocui.ErrUnknownView) {
			return errors.Wrap(err, fmt.Sprintf("Create view for %v", onlineBoxName))
		}
		c.visibleViews = append(c.visibleViews, onlineBoxName)
		onlineBox.Title = "0 online"

		for _, listener := range c.onOnlineBoxOpen {
			listener()
		}
	} else if err == nil {
		c.visibleViews = lo.Without(c.visibleViews, onlineBoxName)
		err := gui.DeleteView(onlineBoxName)
		return errors.Wrap(err, "Delete view")
	}

	return nil
}

// insertNewline insert a new line under the cursor of the given <view>.
func insertNewline(gui *gocui.Gui, view *gocui.View) error {
	view.EditNewLine()
	return nil
}

// scrollUp sets origin position of the <view> internal buffer one row higher.
func scrollUp(gui *gocui.Gui, view *gocui.View) error {
	scroll(-1, view)
	return nil
}

// scrollDown sets origin position of the <view> internal buffer one row lower.
func scrollDown(gui *gocui.Gui, view *gocui.View) error {
	scroll(1, view)
	return nil
}

// scroll sets origin position of the <view> internal buffer <step> rows lower. <step> can be negative. 
func scroll(step int, view *gocui.View) {
	_, sizeY := view.Size()
	originX, originY := view.Origin()

	// If we're at the bottom
	if originY+step > strings.Count(view.ViewBuffer(), "\n")-sizeY-1 {
		view.Autoscroll = true
	} else {
		view.Autoscroll = false
		if originY+step > 0 {
			_ = view.SetOrigin(originX, originY+step)
		}
	}
}

// quit closes the <gui> and returns ErrQuit, making main UI loop exit.
func quit(gui *gocui.Gui, view *gocui.View) error {
	gui.Close()
	return gocui.ErrQuit
}
