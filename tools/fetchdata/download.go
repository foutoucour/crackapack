package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// get opens the MTGJSON file called name. The caller closes the returned body.
func (a app) get(ctx context.Context, name string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/"+name, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", name, err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() // the status is the error; the body is not read
		return nil, fmt.Errorf("download %s: status %d", name, resp.StatusCode)
	}
	return resp.Body, nil
}

// download copies the MTGJSON file called name to path.
func (a app) download(ctx context.Context, name, path string) error {
	body, err := a.get(ctx, name)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }() // read-only: close error is not actionable

	return writeFileAtomic(path, func(w io.Writer) error {
		_, err := io.Copy(w, body)
		return err
	})
}

// downloadPrices streams the MTGJSON price file, keeps the cards in wanted, writes the
// snapshot to path and returns how many cards it priced.
func (a app) downloadPrices(ctx context.Context, wanted map[string]struct{}, path string) (int, error) {
	body, err := a.get(ctx, pricesFile)
	if err != nil {
		return 0, err
	}
	defer func() { _ = body.Close() }() // read-only: close error is not actionable

	zr, err := gzip.NewReader(body)
	if err != nil {
		return 0, fmt.Errorf("decompress %s: %w", pricesFile, err)
	}
	snapshot, err := extractPrices(zr, wanted)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", pricesFile, err)
	}
	err = writeFileAtomic(path, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(snapshot)
	})
	return len(snapshot), err
}

// cardUUIDs returns the UUID of every card in the MTGJSON set file at path.
func cardUUIDs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open catalogue: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only: close error is not actionable

	var set struct {
		Data struct {
			Cards []struct {
				UUID string `json:"uuid"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.NewDecoder(f).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode catalogue %s: %w", path, err)
	}
	uuids := make([]string, len(set.Data.Cards))
	for i, c := range set.Data.Cards {
		uuids[i] = c.UUID
	}
	return uuids, nil
}

// writeFileAtomic writes through a temporary file in the same directory and renames it into
// place, so a failed or interrupted run never leaves a half-written file under path.
func writeFileAtomic(path string, write func(io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if err = write(tmp); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err = os.Chmod(tmp.Name(), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
