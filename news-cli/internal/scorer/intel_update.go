package scorer

import (
"compress/gzip"
"fmt"
"io"
"net/http"
"os"
"path/filepath"
"strings"
)

const (
KEV_URL  = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"
EPSS_URL = "https://epss.cyentia.com/epss_scores-current.csv.gz"
)

func UpdateIntel() error {
configDir, _ := os.UserConfigDir()
intelDir := filepath.Join(configDir, "recon", "intel")
if err := os.MkdirAll(intelDir, 0755); err != nil {
return fmt.Errorf("failed to create intel dir: %w", err)
}

// 1. Update KEV (JSON)
fmt.Printf("📥 Updating CISA KEV Data... ")
if err := downloadFile(KEV_URL, filepath.Join(intelDir, "kev.json")); err != nil {
fmt.Printf("FAILED: %v\n", err)
} else {
fmt.Println("SUCCESS")
}

// 2. Update EPSS (GZ -> CSV)
fmt.Printf("📥 Updating FIRST.org EPSS Data... ")
if err := downloadAndDecompress(EPSS_URL, filepath.Join(intelDir, "epss.csv")); err != nil {
fmt.Printf("FAILED: %v\n", err)
} else {
fmt.Println("SUCCESS")
}

return nil
}

func downloadFile(url, path string) error {
resp, err := http.Get(url)
if err != nil {
return err
}
defer resp.Body.Close()

if resp.StatusCode != 200 {
return fmt.Errorf("status code %d", resp.StatusCode)
}

out, err := os.Create(path)
if err != nil {
return err
}
defer out.Close()

_, err = io.Copy(out, resp.Body)
return err
}

func downloadAndDecompress(url, path string) error {
resp, err := http.Get(url)
if err != nil {
return err
}
defer resp.Body.Close()

if resp.StatusCode != 200 {
return fmt.Errorf("status code %d", resp.StatusCode)
}

var reader io.ReadCloser = resp.Body
if strings.HasSuffix(url, ".gz") {
gzReader, err := gzip.NewReader(resp.Body)
if err != nil {
return err
}
defer gzReader.Close()
reader = gzReader
}

out, err := os.Create(path)
if err != nil {
return err
}
defer out.Close()

_, err = io.Copy(out, reader)
return err
}
