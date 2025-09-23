package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func fetchAvailableVersions() ([]string, error) {
	resp, err := http.Get("https://downloads.gtnewhorizons.com/Multi_mc_downloads/?raw")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	var results []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "Java_17-") {
			continue
		}
		results = append(results, trimmed)
	}
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, nil
}
