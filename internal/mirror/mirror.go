package mirror

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	urlPkg "net/url"
	"regexp"
	"slices"
	"strings"
)

const DefaultRepositoryURL = "https://mirror.ctan.org"
const ctanSitesURL = DefaultRepositoryURL + "/CTAN.sites"

type Mirror struct {
	Region      string
	Country     string
	URL         string
	IsPreferred bool
}

func GetMirrorList() []Mirror {
	regions := []string{"Africa", "Asia", "Europe", "North America", "Oceania", "South America"}

	resp, err := http.DefaultClient.Get(ctanSitesURL)
	if err != nil {
		return []Mirror{}
	}
	defer resp.Body.Close()

	var mirrors []Mirror
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Mirror{}
	}

	var lines []string
	rawLines := strings.SplitSeq(string(body), "\n")

	for line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	currentRegion := ""
	currentMirrorTitle := ""
	currentCountry := ""
	mirrorTitleRegex := regexp.MustCompile(`^(\S+)\s\((.+)\)$`)
	for i, line := range lines {
		nextLine := ""
		if i+1 < len(lines) {
			nextLine = lines[i+1]
		}
		if slices.Contains(regions, line) && nextLine == strings.Repeat("=", len(line)) {
			currentRegion = line
			continue
		}
		if currentRegion == "" {
			continue
		}
		if matches := mirrorTitleRegex.FindStringSubmatch(line); matches != nil {
			currentMirrorTitle = matches[1]
			currentCountry = matches[2]
			continue
		}
		if currentMirrorTitle != "" && currentCountry != "" && strings.HasPrefix(line, "URL: https://") {
			urlValue := strings.TrimPrefix(line, "URL: ")
			url, _ := urlPkg.Parse(urlValue)
			mirrors = append(mirrors, Mirror{
				Region:      currentRegion,
				Country:     currentCountry,
				URL:         urlValue,
				IsPreferred: url.Host == resp.Request.URL.Host,
			})
			currentMirrorTitle = ""
			currentCountry = ""
		}
	}
	slices.SortFunc(mirrors, func(a, b Mirror) int {
		if a.Region != b.Region {
			return strings.Compare(a.Region, b.Region)
		}
		if a.Country != b.Country {
			return strings.Compare(a.Country, b.Country)
		}
		return 0
	})
	return mirrors
}

func ValidateMirrorURL(url string) error {
	if url == "" {
		return fmt.Errorf("Mirror URL cannot be empty")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("Mirror URL must start with http:// or https://")
	}
	testURL, _ := urlPkg.Parse(url)
	testURL.Path, _ = urlPkg.JoinPath(testURL.Path, "systems/texlive/tlnet/tlpkg/texlive.tlpdb")
	resp, err := http.Head(testURL.String())
	if err != nil {
		return fmt.Errorf("Failed to connect to mirror: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Mirror URL is not valid: received status %s (%s)", resp.Status, resp.Request.URL)
	}
	return nil
}

func ResolveMirror(mirrorURL string) (string, error) {
	testURL, _ := url.JoinPath(mirrorURL, "systems/texlive/tlnet/tlpkg/texlive.tlpdb")

	resp, err := http.Head(testURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	exactMirror := strings.TrimSuffix(finalURL, "systems/texlive/tlnet/tlpkg/texlive.tlpdb")

	return exactMirror, nil
}
