package runtimecfg

type DirInfo struct {
	workDir string
	pkgDir  string
}

type WorkDir struct {
	workDir string
}

type PackageDir struct {
	WorkDir
	pkgDir string
}
