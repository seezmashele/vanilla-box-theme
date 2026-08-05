package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// fetchTimeout is per file. The sources are a couple of kilobytes each; a
// request outrunning this is a network that is not going to finish.
const fetchTimeout = 30 * time.Second

// fetchSources vendors the artwork spec/icons.json names.
//
// It is run by hand when a mapping is added, never by `go generate`: generating
// the theme has to work offline and produce the same bytes every time, and a
// build that reaches the network does neither. What it writes is committed.
func fetchSources(root string) error {
	ic, err := readIconSpec(root)
	if err != nil {
		return err
	}

	names, err := ic.names()
	if err != nil {
		return err
	}

	wanted := map[string]bool{}
	for _, phosphor := range names {
		wanted[phosphor] = true
	}

	dir := filepath.Join(root, filepath.FromSlash(phosphorDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	client := &http.Client{Timeout: fetchTimeout}

	for _, phosphor := range sorted(wanted) {
		path := filepath.Join(root, filepath.FromSlash(ic.sourcePath(phosphor)))
		if _, err := os.Stat(path); err == nil {
			continue
		}

		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/assets/%s/%s-%s.svg",
			ic.Source.Repo, ic.Source.Ref, ic.Source.Weight, phosphor, ic.Source.Weight)

		body, err := get(client, url)
		if err != nil {
			return fmt.Errorf("%s: %w", phosphor, err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}

		fmt.Println("vendored", phosphor)
	}

	licence := filepath.Join(dir, "LICENSE")
	if _, err := os.Stat(licence); err != nil {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/LICENSE", ic.Source.Repo, ic.Source.Ref)

		body, err := get(client, url)
		if err != nil {
			return fmt.Errorf("licence: %w", err)
		}
		if err := os.WriteFile(licence, body, 0o644); err != nil {
			return err
		}

		fmt.Println("vendored the licence")
	}

	// Sources nobody maps any more are reported rather than deleted. They are
	// vendored artwork, not build output, and a mapping in progress is a good
	// enough reason for one to sit unused for an afternoon.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "LICENSE" {
			continue
		}

		// A file from another weight is left behind by changing the weight, so
		// it is called what it is rather than unrecognised.
		phosphor, ok := trimSourceName(name, ic.Source.Weight)
		if !ok {
			fmt.Printf("not a %s source: %s\n", ic.Source.Weight, name)

			continue
		}
		if !wanted[phosphor] {
			fmt.Println("unused:", name)
		}
	}

	return nil
}

func get(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// trimSourceName turns a vendored filename back into the Phosphor name it
// carries, so the sweep for unused sources compares like with like.
func trimSourceName(file, weight string) (string, bool) {
	suffix := "-" + weight + ".svg"
	if len(file) <= len(suffix) || file[len(file)-len(suffix):] != suffix {
		return "", false
	}

	return file[:len(file)-len(suffix)], true
}

func sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}
