package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// New returns new logger with log level <lvl> and destination <to>. It also mirroring the log to file.
func New(lvl logrus.Level, to io.Writer) *logrus.Logger {
	logFile, err := os.OpenFile("go_chat_client.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(errors.Wrap(err, "Open or create log file"))
	}
	log := logrus.Logger{
		Out:       to,
		Formatter: formatter{},
		Level:     lvl,
		Hooks:     make(logrus.LevelHooks),
	}
	log.AddHook(newFileHook(logFile))
	return &log
}

// formatter represents logrus formatter.
type formatter struct{}

// Format returns formatted []byte representation of <entry>. Used to implement logrus Formatter interface.
func (f formatter) Format(entry *logrus.Entry) ([]byte, error) {
	buf := &bytes.Buffer{}

	time := entry.Time.Format("15:04:05")
	time = color.GreenString("%v", time)
	buf.WriteString(time)
	buf.WriteByte(' ')

	var levelColor func(a ...any) string
	switch entry.Level {
	case logrus.InfoLevel:
		levelColor = color.New(color.FgGreen).SprintFunc()
	case logrus.WarnLevel:
		levelColor = color.New(color.FgYellow).SprintFunc()
	case logrus.ErrorLevel:
		levelColor = color.New(color.FgRed).SprintFunc()
	case logrus.FatalLevel:
		levelColor = color.New(color.FgRed).SprintFunc()
	case logrus.PanicLevel:
		levelColor = color.New(color.FgRed).SprintFunc()
	case logrus.DebugLevel:
		levelColor = color.New(color.FgBlue).SprintFunc()
	}

	level := strings.ToUpper(entry.Level.String())
	buf.WriteString(levelColor(level))
	buf.WriteByte(' ')

	buf.WriteString(entry.Message)

	buf.WriteString(formatFields(entry.Data, levelColor))

	buf.WriteByte('\n')

	return buf.Bytes(), nil
}

// formatFields returns formatted <fields> colored with <levelColor> as a string.
func formatFields(fields logrus.Fields, levelColor func(a ...any) string) string {
	var sb strings.Builder
	keys := lo.Keys(fields)
	slices.Sort(keys)
	for _, key := range keys {
		val := fields[key]
		if levelColor != nil {
			key = levelColor(key)
		}
		sb.WriteString(fmt.Sprintf(" %s=%+v", key, val))
	}
	return sb.String()
}

// fileHook represents logrus file hook.
type fileHook struct {
	file *os.File
}

// newFileHook returns new logrus file hook.
func newFileHook(file *os.File) fileHook {
	return fileHook{file: file}
}

// Levels returns which levels to fire the hook at. Used to implement logrus Hook interface.
func (h fileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is executed when the hook runs, writing formatted <entry> to file. Used to implement logrus Hook interface.
func (h fileHook) Fire(entry *logrus.Entry) error {
	time := entry.Time.Format("15:04:05")
	level := strings.ToUpper(entry.Level.String())
	msg := fmt.Sprintf("%s %s %s%s\n", time, level, entry.Message, formatFields(entry.Data, nil))
	_, err := h.file.WriteString(msg)
	return err
}

// chatUIHook represents logrus chat UI hook.
type chatUIHook struct {
	gui *gocui.Gui
}

// newFileHook returns new logrus chat UI hook.
func NewChatUIHook(gui *gocui.Gui) chatUIHook {
	return chatUIHook{gui: gui}
}

// Levels returns which levels to fire the hook at. Used to implement logrus Hook interface.
func (h chatUIHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is executed when the hook runs, updating chat GUI to reflect changes in internal buffers of every view in it.
// Used to implement logrus Hook interface.
func (h chatUIHook) Fire(entry *logrus.Entry) error {
	h.gui.Update(func(g *gocui.Gui) error {
		return nil
	})
	return nil
}
