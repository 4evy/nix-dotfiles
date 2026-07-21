package process

import (
	"os"

	"github.com/4evy/dotfiles/internal/common/fileutil"
)

func osWriteFileExecutable(path string, data []byte) error {
	return os.WriteFile(path, data, fileutil.DefaultFilePerm|fileutil.ExecutablePerm)
}
