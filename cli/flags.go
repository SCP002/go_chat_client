package cli

import (
	"github.com/cockroachdb/errors"
	goFlags "github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
)

// Flags represents command line flags.
type Flags struct {
	Version  bool         `short:"v" long:"version"  description:"Print the program version"`
	LogLevel logrus.Level `short:"l" long:"logLevel" description:"Logging level. Can be from 0 (least verbose) to 6 (most verbose)"`
}

// Parse returns a structure initialized with command line arguments and error if parsing failed.
func Parse() (Flags, error) {
	flags := Flags{LogLevel: logrus.InfoLevel} // Set defaults
	parser := goFlags.NewParser(&flags, goFlags.Options(goFlags.Default))
	_, err := parser.Parse()
	return flags, errors.Wrap(err, "Parse CLI arguments")
}

// IsErrOfType returns true if <err> is of type <t>.
func IsErrOfType(err error, t goFlags.ErrorType) bool {
	goFlagsErr := &goFlags.Error{}
	if ok := errors.As(err, &goFlagsErr); ok && goFlagsErr.Type == t {
		return true
	}
	return false
}
