package embedded

import (
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed qscanner-linux-amd64.gz
var qscannerFS embed.FS

var cachedPath string

func ExtractQScanner() (string, error) {
	if cachedPath != "" {
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
	}

	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("qscanner only runs on Linux (current OS: %s)", runtime.GOOS)
	}

	if runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("qscanner only runs on amd64 (current arch: %s)", runtime.GOARCH)
	}

	gzData, err := qscannerFS.ReadFile("qscanner-linux-amd64.gz")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded qscanner: %w", err)
	}

	hash := sha256.Sum256(gzData)
	hashStr := hex.EncodeToString(hash[:8])

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}

	qscanDir := filepath.Join(cacheDir, "qscan")
	if err := os.MkdirAll(qscanDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	binaryPath := filepath.Join(qscanDir, fmt.Sprintf("qscanner-%s", hashStr))

	if info, err := os.Stat(binaryPath); err == nil {
		if info.Mode()&0111 != 0 {
			cachedPath = binaryPath
			return binaryPath, nil
		}
	}

	tmpFile, err := os.CreateTemp(qscanDir, "qscanner-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	gzReader, err := gzip.NewReader(io.NopCloser(
		&bytesReader{data: gzData},
	))
	if err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}

	if _, err := io.Copy(tmpFile, gzReader); err != nil {
		tmpFile.Close()
		gzReader.Close()
		return "", fmt.Errorf("failed to extract qscanner: %w", err)
	}

	gzReader.Close()
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return "", fmt.Errorf("failed to set executable permission: %w", err)
	}

	if err := os.Rename(tmpPath, binaryPath); err != nil {
		if err2 := copyFile(tmpPath, binaryPath); err2 != nil {
			return "", fmt.Errorf("failed to move qscanner to cache: %w", err)
		}
		os.Remove(tmpPath)
	}
	tmpPath = ""

	cachedPath = binaryPath
	return binaryPath, nil
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func GetEmbeddedVersion() string {
	gzData, err := qscannerFS.ReadFile("qscanner-linux-amd64.gz")
	if err != nil {
		return "unknown"
	}
	hash := sha256.Sum256(gzData)
	return hex.EncodeToString(hash[:8])
}
