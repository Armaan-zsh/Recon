package scorer

import (
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

//go:embed cisa_kev.json
var embeddedKEV []byte

type KEVEntry struct {
	CVEID string `json:"cveID"`
}

type KEVData struct {
	Vulnerabilities []KEVEntry `json:"vulnerabilities"`
}

var (
	KEVLookup  map[string]bool
	EPSSLookup map[string]float64
)

func LoadIntel() error {
	KEVLookup = make(map[string]bool)
	EPSSLookup = make(map[string]float64)

	// 1. Load KEV
	var kevData KEVData
	configDir, _ := os.UserConfigDir()
	localKEVPath := filepath.Join(configDir, "recon", "intel", "kev.json")
	
	data := embeddedKEV
	if localData, err := os.ReadFile(localKEVPath); err == nil {
		data = localData
	}

	if err := json.Unmarshal(data, &kevData); err == nil {
		for _, v := range kevData.Vulnerabilities {
			KEVLookup[v.CVEID] = true
		}
	}

	// 2. Load EPSS
	localEPSSPath := filepath.Join(configDir, "recon", "intel", "epss.csv")
	if f, err := os.Open(localEPSSPath); err == nil {
		defer f.Close()
		reader := csv.NewReader(f)
		reader.ReuseRecord = true
		// Skip header if it exists
		if _, err := reader.Read(); err == nil {
			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if len(record) >= 2 {
					if score, err := strconv.ParseFloat(record[1], 64); err == nil {
						EPSSLookup[record[0]] = score
					}
				}
			}
		}
	}

	return nil
}

func GetKEVScoreBoost(cve string) int {
	if KEVLookup[cve] {
		return 100
	}
	return 0
}

func GetEPSSScoreBoost(cve string) int {
	if score, ok := EPSSLookup[cve]; ok {
		return int(score * 50) // Scale 0-1 to 0-50
	}
	return 0
}
