package main

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func fetchAvailableVersions() ([]string, error) {
	resp, err := http.Get(gtnhDownloadsListingURL)
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
	infos := make([]releaseInfo, 0, len(results))
	for _, r := range results {
		infos = append(infos, parseReleaseInfo(r))
	}
	sort.SliceStable(infos, func(i, j int) bool {
		cmp := compareSemver(infos[i].baseVersion, infos[j].baseVersion)
		if cmp != 0 {
			return cmp > 0
		}
		if infos[i].releaseType != infos[j].releaseType {
			return infos[i].releaseType < infos[j].releaseType
		}
		if infos[i].preNumber != infos[j].preNumber {
			return infos[i].preNumber > infos[j].preNumber
		}
		return infos[i].original > infos[j].original
	})
	ordered := make([]string, len(infos))
	for i, info := range infos {
		ordered[i] = info.original
	}
	return ordered, nil
}

type releaseType int

const (
	releaseStable releaseType = iota
	releaseRC
	releaseBeta
	releaseUnknown
)

type releaseInfo struct {
	original    string
	baseVersion string
	releaseType releaseType
	preNumber   int
}

func parseReleaseInfo(name string) releaseInfo {
	info := releaseInfo{original: name, releaseType: releaseStable}
	fileName := name
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		fileName = name[idx+1:]
	}
	lowerName := strings.ToLower(fileName)
	lowerName = strings.TrimSuffix(lowerName, ".zip")
	versionSegment := lowerName
	if idx := strings.Index(versionSegment, "gt_new_horizons_"); idx != -1 {
		versionSegment = versionSegment[idx+len("gt_new_horizons_"):]
	}
	if idx := strings.Index(versionSegment, "_java"); idx != -1 {
		versionSegment = versionSegment[:idx]
	}
	base := versionSegment
	suffix := ""
	if dash := strings.Index(versionSegment, "-"); dash != -1 {
		base = versionSegment[:dash]
		suffix = versionSegment[dash+1:]
	}
	if base == "" {
		base = "0.0.0"
		info.releaseType = releaseUnknown
	}
	info.baseVersion = base
	if suffix == "" {
		return info
	}
	lowerSuffix := strings.ToLower(suffix)
	switch {
	case strings.Contains(lowerSuffix, "beta"):
		info.releaseType = releaseBeta
		info.preNumber = extractPreNumber(lowerSuffix, "beta")
	case strings.Contains(lowerSuffix, "rc"):
		info.releaseType = releaseRC
		info.preNumber = extractPreNumber(lowerSuffix, "rc")
	default:
		info.releaseType = releaseUnknown
	}
	return info
}

func extractPreNumber(s, keyword string) int {
	idx := strings.Index(s, keyword)
	if idx == -1 {
		return 0
	}
	rest := s[idx+len(keyword):]
	rest = strings.TrimLeft(rest, "-_")
	numStr := strings.Builder{}
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		numStr.WriteRune(r)
	}
	if numStr.Len() == 0 {
		return 0
	}
	n, err := strconv.Atoi(numStr.String())
	if err != nil {
		return 0
	}
	return n
}

func compareSemver(a, b string) int {
	aparts := strings.Split(a, ".")
	bparts := strings.Split(b, ".")
	maxLen := len(aparts)
	if len(bparts) > maxLen {
		maxLen = len(bparts)
	}
	for i := 0; i < maxLen; i++ {
		ai := partOrZero(aparts, i)
		bi := partOrZero(bparts, i)
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

func partOrZero(parts []string, idx int) int {
	if idx >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[idx])
	if err != nil {
		return 0
	}
	return n
}
