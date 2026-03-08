package resolver

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
