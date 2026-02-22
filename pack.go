//go:build ignore
// +build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PluginManifest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func main() {
	// Read manifest
	mf, err := os.ReadFile("plugin.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading plugin.json: %v\n", err)
		os.Exit(1)
	}
	var manifest PluginManifest
	if err := json.Unmarshal(mf, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing plugin.json: %v\n", err)
		os.Exit(1)
	}

	bundleName := fmt.Sprintf("%s-%s.tar.gz", manifest.ID, manifest.Version)
	fmt.Printf("Creating %s...\n", bundleName)

	outFile, err := os.Create(bundleName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tw := tar.NewWriter(gzWriter)
	defer tw.Close()

	pluginDir := manifest.ID

	// Files to include
	filesToPack := []struct {
		src     string
		dst     string
		mode    int64
	}{
		{"plugin.json", pluginDir + "/plugin.json", 0644},
	}

	// Add server binaries
	serverBinaries := []string{
		"server/dist/plugin-linux-amd64",
		"server/dist/plugin-linux-arm64",
		"server/dist/plugin-darwin-amd64",
		"server/dist/plugin-darwin-arm64",
		"server/dist/plugin-windows-amd64.exe",
	}
	for _, bin := range serverBinaries {
		if _, err := os.Stat(bin); err == nil {
			filesToPack = append(filesToPack, struct {
				src  string
				dst  string
				mode int64
			}{bin, pluginDir + "/" + bin, 0755})
		}
	}

	// Add webapp bundle
	webappBundle := "webapp/dist/main.js"
	if _, err := os.Stat(webappBundle); err == nil {
		filesToPack = append(filesToPack, struct {
			src  string
			dst  string
			mode int64
		}{webappBundle, pluginDir + "/" + webappBundle, 0644})
	}

	// Add assets if present
	if entries, err := os.ReadDir("assets"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				src := filepath.Join("assets", entry.Name())
				filesToPack = append(filesToPack, struct {
					src  string
					dst  string
					mode int64
				}{src, pluginDir + "/assets/" + entry.Name(), 0644})
			}
		}
	}

	// Collect directories
	dirs := map[string]bool{}
	for _, f := range filesToPack {
		parts := strings.Split(filepath.ToSlash(f.dst), "/")
		for i := 1; i < len(parts); i++ {
			dir := strings.Join(parts[:i], "/") + "/"
			dirs[dir] = true
		}
	}

	// Write directory entries
	dirList := make([]string, 0, len(dirs))
	for d := range dirs {
		dirList = append(dirList, d)
	}
	// Sort for deterministic output
	for i := 0; i < len(dirList); i++ {
		for j := i + 1; j < len(dirList); j++ {
			if dirList[i] > dirList[j] {
				dirList[i], dirList[j] = dirList[j], dirList[i]
			}
		}
	}
	for _, d := range dirList {
		tw.WriteHeader(&tar.Header{
			Name:     d,
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
	}

	// Write files
	count := 0
	for _, f := range filesToPack {
		data, err := os.ReadFile(f.src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", f.src, err)
			continue
		}

		err = tw.WriteHeader(&tar.Header{
			Name:     filepath.ToSlash(f.dst),
			Size:     int64(len(data)),
			Mode:     f.mode,
			Typeflag: tar.TypeReg,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing header for %s: %v\n", f.dst, err)
			continue
		}

		if _, err := tw.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", f.dst, err)
			continue
		}
		count++
	}



	fmt.Printf("Packed %d files into %s\n", count, bundleName)
}
