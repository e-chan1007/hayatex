package utils

import (
	"net/http"
	"net/url"
	"strings"
)

func ResolveMirror(mirrorURL string) (string, error) {
	testURL, _ := url.JoinPath(mirrorURL, "tlpkg/texlive.tlpdb")

	resp, err := http.Head(testURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	exactMirror := strings.TrimSuffix(finalURL, "tlpkg/texlive.tlpdb")

	return exactMirror, nil
}
