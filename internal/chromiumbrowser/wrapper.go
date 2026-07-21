package chromiumbrowser

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

const (
	commandJSONPlaceholder   = "__COMMAND_JSON__"
	flagsFileJSONPlaceholder = "__FLAGS_FILE_JSON__"
)

//go:embed scripts/wrapper.py
var wrapperScriptTemplate string

func writeWrapper(target, launcher, flagsFile string, options *InstallOptions) error {
	return WriteWrapper(target, launcher, flagsFile, options.Flags, options.extraWrapperFlags)
}

func WriteWrapper(target, launcher, flagsFile string, flags, extraFlags []string) error {
	args := slices.Concat([]string{launcher}, flags, extraFlags)
	content := renderWrapperScript(args, flagsFile)
	return fileutil.WriteExecutable(target, []byte(content))
}

func renderWrapperScript(args []string, flagsFile string) string {
	encoded, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	flagsFileJSON, err := json.Marshal(flagsFile)
	if err != nil {
		panic(err)
	}
	return strings.NewReplacer(
		commandJSONPlaceholder, string(encoded),
		flagsFileJSONPlaceholder, string(flagsFileJSON),
	).Replace(wrapperScriptTemplate)
}

func replaceSymlink(oldname, newname string) error {
	if err := os.Remove(newname); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(oldname, newname)
}
