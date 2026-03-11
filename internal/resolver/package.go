package resolver

import (
	"fmt"
	"sort"
	"strings"
)

// The list of contained files and their total size for a package
type TLPackageFiles struct {
	Files []string
	Size  uint64
}

// Information about the container file for a package, including its size and checksum
type TLContainerInfo struct {
	Size     uint64
	Checksum string
}

// A single package entry in the TeX Live database
type TLPackage struct {
	Name         string
	Category     string
	Revision     string
	ShortDesc    string
	Relocated    bool
	Depends      []string
	ArchDepends  map[string][]string
	BinFiles     map[string]*TLPackageFiles
	DocFiles     *TLPackageFiles
	RunFiles     *TLPackageFiles
	SrcFiles     *TLPackageFiles
	Executes     []string
	Container    *TLContainerInfo
	DocContainer *TLContainerInfo
	SrcContainer *TLContainerInfo
}

// The entire TeX Live package database, mapping package names to their corresponding TLPackage structs
type TLDatabase map[string]*TLPackage

func (p *TLPackage) ToString() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "name %s\n", p.Name)

	if p.Category != "" {
		fmt.Fprintf(&sb, "category %s\n", p.Category)
	}
	if p.Revision != "" {
		fmt.Fprintf(&sb, "revision %s\n", p.Revision)
	}
	if p.ShortDesc != "" {
		fmt.Fprintf(&sb, "shortdesc %s\n", p.ShortDesc)
	}

	if p.Container != nil {
		if p.Container.Size != 0 {
			fmt.Fprintf(&sb, "containersize %d\n", p.Container.Size)
		}
		if p.Container.Checksum != "" {
			fmt.Fprintf(&sb, "containerchecksum %s\n", p.Container.Checksum)
		}
	}
	if p.SrcContainer != nil {
		if p.SrcContainer.Size != 0 {
			fmt.Fprintf(&sb, "srccontainersize %d\n", p.SrcContainer.Size)
		}
		if p.SrcContainer.Checksum != "" {
			fmt.Fprintf(&sb, "srccontainerchecksum %s\n", p.SrcContainer.Checksum)
		}
	}
	if p.DocContainer != nil {
		if p.DocContainer.Size != 0 {
			fmt.Fprintf(&sb, "doccontainersize %d\n", p.DocContainer.Size)
		}
		if p.DocContainer.Checksum != "" {
			fmt.Fprintf(&sb, "doccontainerchecksum %s\n", p.DocContainer.Checksum)
		}
	}

	for _, d := range p.Depends {
		fmt.Fprintf(&sb, "depend %s\n", d)
	}
	for arch, deps := range p.ArchDepends {
		for _, d := range deps {
			fmt.Fprintf(&sb, "depend.%s %s\n", arch, d)
		}
	}
	for _, e := range p.Executes {
		fmt.Fprintf(&sb, "execute %s\n", e)
	}

	for arch, bf := range p.BinFiles {
		if bf == nil {
			continue
		}
		if bf.Size != 0 {
			fmt.Fprintf(&sb, "binfiles arch=%s size=%d\n", arch, bf.Size)
		} else {
			fmt.Fprintf(&sb, "binfiles arch=%s\n", arch)
		}
		for _, f := range bf.Files {
			f = strings.ReplaceAll(f, "RELOC", "texmf-dist")
			fmt.Fprintf(&sb, " %s\n", f)
		}
	}

	if p.RunFiles != nil {
		if p.RunFiles.Size != 0 {
			fmt.Fprintf(&sb, "runfiles size=%d\n", p.RunFiles.Size)
		} else {
			fmt.Fprintln(&sb, "runfiles")
		}
		for _, f := range p.RunFiles.Files {
			f = strings.ReplaceAll(f, "RELOC", "texmf-dist")
			fmt.Fprintf(&sb, " %s\n", f)
		}
	}
	if p.DocFiles != nil {
		if p.DocFiles.Size != 0 {
			fmt.Fprintf(&sb, "docfiles size=%d\n", p.DocFiles.Size)
		} else {
			fmt.Fprintln(&sb, "docfiles")
		}
		for _, f := range p.DocFiles.Files {
			f = strings.ReplaceAll(f, "RELOC", "texmf-dist")
			fmt.Fprintf(&sb, " %s\n", f)
		}
	}
	if p.SrcFiles != nil {
		if p.SrcFiles.Size != 0 {
			fmt.Fprintf(&sb, "srcfiles size=%d\n", p.SrcFiles.Size)
		} else {
			fmt.Fprintln(&sb, "srcfiles")
		}
		for _, f := range p.SrcFiles.Files {
			f = strings.ReplaceAll(f, "RELOC", "texmf-dist")
			fmt.Fprintf(&sb, " %s\n", f)
		}
	}

	fmt.Fprintln(&sb)

	return sb.String()
}

func (db TLDatabase) ToString() string {
	var sb strings.Builder
	keys := make([]string, 0, len(db))
	for k := range db {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		p := db[k]
		if p == nil {
			continue
		}
		sb.WriteString(p.ToString())
	}

	return sb.String()
}
