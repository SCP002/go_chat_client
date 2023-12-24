package stdin

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// AskServerAddress returns address of server to connect to, taking it from standard input.
func AskServerAddress(log *logrus.Logger) string {
	return ask(log, true, "Enter server address in format of 'host:port': ", func(input string) bool {
		return input == ""
	})
}

// AskServerAddress returns true if need to establish secure connection to server, taking y/n value from standard input.
func AskTLSMode(log *logrus.Logger) *bool {
	tls := askYesNo(log, "Connect to server using TLS protocol? (y/n): ")
	return &tls
}

// AskNickname returns nickname to use to log in, taking it from standard input.
func AskNickname(log *logrus.Logger) string {
	return ask(log, true, "Enter your nickname: ", func(input string) bool {
		if input == "" {
			return true
		}
		maxSymbols := 20
		if len(input) > maxSymbols {
			log.Warnf("Nicknames with length > %v symbols are not allowed", maxSymbols)
			return true
		}
		return false
	})
}

// askYesNo returns true if user input is 'y' or 'Y'. If user types neither 'y', 'Y', 'n' or 'N', it asks again.
func askYesNo(log *logrus.Logger, prompt string) bool {
	answer := ask(log, true, prompt, func(input string) bool {
		if input == "" {
			return true
		}
		input = strings.ToLower(input)
		if input != "y" && input != "n" {
			return true
		}
		return false
	})
	return lo.Ternary(answer == "y", true, false)
}

// ask returns user input, preliminarily printing <prompt>. It runs forever until read is successfull and <callback>
// returns false. If <trim> is true, trim space from user input before passing it to <callback>.
func ask(log *logrus.Logger, trim bool, prompt string, callback func(string) bool) string {
	for {
		fmt.Print(prompt)
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if trim {
			input = strings.TrimSpace(input)
		}
		if callback(input) {
			continue
		}
		if err == nil {
			return input
		}
		log.Error(errors.Wrap(err, "Read from standard input"))
	}
}
