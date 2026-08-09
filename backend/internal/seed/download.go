package seed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Dataset download locations.
const (
	// CEDICTURL is the CC-CEDICT export from MDBG (CC BY-SA 4.0 — see NOTICES.md).
	CEDICTURL = "https://www.mdbg.net/chinese/export/cedict/cedict_1_0_ts_utf-8_mdbg.txt.gz"
	// StrokesURL is the hanzi-writer-data npm tarball (Arphic Public License — see NOTICES.md).
	StrokesURL = "https://registry.npmjs.org/hanzi-writer-data/-/hanzi-writer-data-2.0.1.tgz"
	// DecompositionsURL is Make Me a Hanzi dictionary.txt, pinned to one commit (LGPL-3.0-or-later).
	DecompositionsURL = "https://raw.githubusercontent.com/skishore/makemeahanzi/" +
		"bddc96d41bef78427ed0e034e9f7e31d71fd1b92/dictionary.txt"
)

const downloadTimeout = 5 * time.Minute

// DownloadCached fetches url into dir (keyed by filename) unless the file
// already exists, and returns the local path.
func DownloadCached(ctx context.Context, dir, filename, url string) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	dest := filepath.Join(dir, filename)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %w", url, fmt.Errorf("status %s", resp.Status)) //nolint:err113 // status detail
	}

	tmp := dest + ".tmp"

	out, err := os.Create(tmp) //nolint:gosec // path is server-controlled config
	if err != nil {
		return "", fmt.Errorf("create %s: %w", tmp, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)

		return "", fmt.Errorf("write %s: %w", tmp, err)
	}

	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("finalize %s: %w", dest, err)
	}

	return dest, nil
}
