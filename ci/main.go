package main

import (
	"os"
	"path/filepath"
)

// FIXME
// https://github.com/dagger/dagger/issues/5914
func hostPath(elem ...string) string {
	wd, err := os.Getwd()

	if err != nil {
		panic(err)
	}

	return filepath.Join(wd, "..", filepath.Join(elem...))
}

func hostDirectory(elem ...string) *Directory {
	return dag.Host().Directory(hostPath(elem...))
}

type CI struct{}

func (*CI) Vector() *VectorTarget {
	return &VectorTarget{
		HelmChartPath: hostPath("argocd", "helm", "vector"),
	}
}

func (*CI) Docs() *DocsTarget {
	return &DocsTarget{
		Directory: hostDirectory("docs"),
	}
}
