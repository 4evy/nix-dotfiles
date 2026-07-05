package process

import "os"

func osWriteFileExecutable(path string, data []byte) error {
	return os.WriteFile(path, data, 0o755)
}
