package main

import (
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	tlpdb, err := resolver.RetrieveTLDatabase("https://ftp.jaist.ac.jp/pub/CTAN/systems/texlive/tlnet")
	if err != nil {
		panic(err)
	}

	deps := resolver.ResolveDependencies([]string{"collection-basic"}, tlpdb, "x86_64-linux")

	accContainerSize := uint64(0)
	accSrcContainerSize := uint64(0)
	accDocContainerSize := uint64(0)
	for name := range deps {
		println(name)
		accContainerSize += deps[name].Container.Size
		accSrcContainerSize += deps[name].SrcContainer.Size
		accDocContainerSize += deps[name].DocContainer.Size
	}
	println("Total Download size:", utils.FormatBytes(accContainerSize, "KB"), utils.FormatBytes(accSrcContainerSize, "KB"), utils.FormatBytes(accDocContainerSize, "KB"))
}
