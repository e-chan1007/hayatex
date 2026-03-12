package resolver

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Finds an key=value option in the given parts and returns the value if found
func findOption(parts []string, key string) (string, bool) {
	for _, part := range parts {
		if after, ok := strings.CutPrefix(part, key+"="); ok {
			return after, true
		}
	}
	return "", false
}

// Retrieves the TLPDB file from the specified mirror URL and parses it into a TLDatabase struct.
func RetrieveTLDatabase(mirrorURL string) (*TLDatabase, error) {
	mirrorURL, _ = url.JoinPath(mirrorURL, "tlpkg/texlive.tlpdb")
	res, err := http.Get(mirrorURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve TLPDB: %s", res.Status)
	}

	db, err := parseTLPDB(res.Body)
	return db, err
}

// Creates a new TLPackage with initialized fields
func newTLPackage() *TLPackage {
	return &TLPackage{
		Relocated:   false,
		ArchDepends: make(map[string][]string),
		BinFiles:    make(map[string]*TLPackageFiles),
		RunFiles:    &TLPackageFiles{},
		DocFiles:    &TLPackageFiles{},
		SrcFiles:    &TLPackageFiles{},
		Container: &TLContainerInfo{
			Size:     0,
			Checksum: "",
		},
		DocContainer: &TLContainerInfo{
			Size:     0,
			Checksum: "",
		},
		SrcContainer: &TLContainerInfo{
			Size:     0,
			Checksum: "",
		},
	}
}

// Parses the TLPDB file at the given path and returns a TLDatabase containing all package information.
func parseTLPDB(reader io.Reader) (*TLDatabase, error) {
	db := make(TLDatabase)
	scanner := bufio.NewScanner(reader)

	var currentPkg *TLPackage = newTLPackage()
	var currentField string
	var currentArch string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if currentPkg != nil && currentPkg.Name != "" {
				db[currentPkg.Name] = currentPkg
			}
			currentPkg = newTLPackage()
			currentField = ""
			continue
		}

		if strings.HasPrefix(line, " ") {
			if currentPkg != nil && currentField != "" {
				trimmed := strings.TrimSpace(line)
				switch currentField {
				case "runfiles":
					currentPkg.RunFiles.Files = append(currentPkg.RunFiles.Files, trimmed)
				case "docfiles":
					currentPkg.DocFiles.Files = append(currentPkg.DocFiles.Files, trimmed)
				case "srcfiles":
					currentPkg.SrcFiles.Files = append(currentPkg.SrcFiles.Files, trimmed)
				case "binfiles":
					binFiles := currentPkg.BinFiles[currentArch]
					binFiles.Files = append(binFiles.Files, trimmed)
					currentPkg.BinFiles[currentArch] = binFiles
				}
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		key := parts[0]
		options := strings.Fields(parts[1])
		var val string
		if len(parts) > 1 {
			val = strings.Join(parts[1:], " ")
		}

		switch {
		case key == "name":
			currentPkg.Name = val
		case key == "category":
			currentPkg.Category = val
		case key == "revision":
			currentPkg.Revision = val
		case key == "shortdesc":
			currentPkg.ShortDesc = val
		case key == "relocated":
			currentPkg.Relocated = true
		case key == "containersize":
			size, _ := strconv.ParseUint(val, 10, 64)
			currentPkg.Container.Size = size
		case key == "containerchecksum":
			currentPkg.Container.Checksum = val
		case key == "srccontainersize":
			size, _ := strconv.ParseUint(val, 10, 64)
			currentPkg.SrcContainer.Size = size
		case key == "srccontainerchecksum":
			currentPkg.SrcContainer.Checksum = val
		case key == "doccontainersize":
			size, _ := strconv.ParseUint(val, 10, 64)
			currentPkg.DocContainer.Size = size
		case key == "doccontainerchecksum":
			currentPkg.DocContainer.Checksum = val

		case key == "depend":
			currentPkg.Depends = append(currentPkg.Depends, val)
		case strings.HasPrefix(key, "depend."):
			arch := strings.TrimPrefix(key, "depend.")
			currentPkg.ArchDepends[arch] = append(currentPkg.ArchDepends[arch], val)
		case key == "execute":
			currentPkg.Executes = append(currentPkg.Executes, val)
		case key == "runfiles":
			currentField = "runfiles"
			if sizeStr, ok := findOption(options, "size"); ok {
				size, _ := strconv.ParseUint(sizeStr, 10, 64)
				currentPkg.RunFiles.Size = size
			}
		case key == "docfiles":
			currentField = "docfiles"
			if sizeStr, ok := findOption(options, "size"); ok {
				size, _ := strconv.ParseUint(sizeStr, 10, 64)
				currentPkg.DocFiles.Size = size
			}
		case key == "srcfiles":
			currentField = "srcfiles"
			if sizeStr, ok := findOption(options, "size"); ok {
				size, _ := strconv.ParseUint(sizeStr, 10, 64)
				currentPkg.SrcFiles.Size = size
			}
		case strings.HasPrefix(key, "binfiles"):
			currentField = "binfiles"
			if arch, ok := findOption(options, "arch"); ok {
				currentArch = arch
				if _, exists := currentPkg.BinFiles[currentArch]; !exists {
					currentPkg.BinFiles[currentArch] = &TLPackageFiles{}
				}
			}
			if sizeStr, ok := findOption(options, "size"); ok {
				size, _ := strconv.ParseUint(sizeStr, 10, 64)
				if currentArch != "" {
					currentPkg.BinFiles[currentArch].Size = size
				}
			}
		default:
			currentField = ""
		}
	}

	return &db, scanner.Err()
}
