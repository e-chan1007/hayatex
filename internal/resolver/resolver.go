package resolver

// Resolves dependencies for the given root package names and architecture, returning a map of package names to their corresponding TLPackage structs.
func ResolveDependencies(rootNames []string, db *TLDatabase, arch string) *TLPackageList {
	resolved := make(TLPackageList)
	var walk func(name string)

	walk = func(name string) {
		if _, ok := resolved[name]; ok {
			return
		}

		pkg, ok := db.Packages[name]
		if !ok {
			return
		}

		resolved[name] = pkg

		archPkgName := pkg.Name + "." + arch
		if _, ok := db.Packages[archPkgName]; ok {
			walk(archPkgName)
		}

		for _, dep := range pkg.Depends {
			walk(dep)
		}

		if deps, ok := pkg.ArchDepends[arch]; ok {
			for _, dep := range deps {
				walk(dep)
			}
		}
	}

	for _, name := range rootNames {
		walk(name)
	}

	return &resolved
}
