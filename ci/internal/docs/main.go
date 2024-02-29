package main

import (
	"context"
	"fmt"
	"strings"
)

const (
	KrokiImageRegistry   string = "docker.io"
	KrokiImageRepository string = "yuzutech/kroki"
	KrokiImageTag        string = "0.24.1"
	KrokiImageDigest     string = "sha256:e0cfbf7d53a6aa1aa5e62a0e4281b4f39e44a63cfa4ba5410ce4178201922e4e"

	HugoVersionFilename string = ".hugo-version"

	rubyCacheDir string = "/var/cache/ruby"
	nodeCacheDir string = "/var/cache/node"
)

type Docs struct {
	Directory   *Directory
	HugoVersion string
}

func New(ctx context.Context, directory *Directory) (*Docs, error) {
	hugoVersion, err := directory.File(HugoVersionFilename).Contents(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to read Hugo version from %q: %w", HugoVersionFilename, err)
	}

	docs := &Docs{
		Directory:   directory,
		HugoVersion: strings.TrimSpace(hugoVersion),
	}

	return docs, nil
}

func entrypoint(command string) []string {
	return []string{"sh", "-c", "npm ci && exec " + command + " \"$@\"", command}
}

func (docs *Docs) Container(ctx context.Context, krokiImageRegistry Optional[string], krokiImageRepository Optional[string], krokiImageTag Optional[string], krokiImageDigest Optional[string]) *Container {
	_krokiImageRegistry := krokiImageRegistry.GetOr(KrokiImageRegistry)
	_krokiImageRepository := krokiImageRepository.GetOr(KrokiImageRepository)
	_krokiImageTag := krokiImageTag.GetOr(KrokiImageTag)
	_krokiImageDigest := krokiImageTag.GetOr(KrokiImageDigest)

	container := dag.Hugo(docs.HugoVersion).Container(HugoContainerOpts{
		Packages: []string{
			"ruby-devel",
			"redhat-rpm-config",
			"rubygem-bundler",
		},
	})

	file := func(path string) *File {
		return docs.Directory.
			File(path)
	}

	container = container.
		WithEntrypoint([]string{"sh", "-c"}).
		WithMountedFile("Gemfile", file("Gemfile")).
		WithMountedFile("Gemfile.lock", file("Gemfile.lock")).
		WithMountedCache(rubyCacheDir, dag.CacheVolume("ruby")).
		WithEnvVariable("BUNDLE_CACHE_PATH", rubyCacheDir+"/bundle").
		WithEnvVariable("BUNDLE_SYSTEM", "true").
		WithEnvVariable("BUNDLE_SILENCE_ROOT_WARNING", "true").
		WithExec([]string{"bundle install"}).
		WithMountedFile("package.json", file("package.json")).
		WithMountedFile("package-lock.json", file("package-lock.json")).
		WithMountedCache(nodeCacheDir, dag.CacheVolume("node")).
		WithEnvVariable("NPM_CONFIG_CACHE", nodeCacheDir+"/npm").
		WithExec([]string{"npm ci"}).
		WithEntrypoint(entrypoint("hugo"))

	kroki := dag.Container().
		From(_krokiImageRegistry + "/" + _krokiImageRepository + ":" + _krokiImageTag + "@" + _krokiImageDigest).
		WithExposedPort(8000).
		AsService()

	container = container.
		WithServiceBinding("kroki", kroki)

	return container
}

func (docs *Docs) _Container(ctx context.Context) *Container {
	return docs.Container(ctx, OptEmpty[string](), OptEmpty[string](), OptEmpty[string](), OptEmpty[string]())
}

func (docs *Docs) Shell(ctx context.Context) *Container {
	shell := docs._Container(ctx).
		WithMountedDirectory(".", docs.Directory).
		WithEntrypoint(entrypoint("bash"))

	return shell
}

func (docs *Docs) Build(ctx context.Context, baseURL string, args Optional[[]string]) *Directory {
	_args := args.GetOr([]string{"--cleanDestinationDir", "--minify"})

	output := dag.Hugo(docs.HugoVersion).Build(docs.Directory, baseURL, HugoBuildOpts{
		Container: docs._Container(ctx),
		Args:      _args,
	})

	return output
}

func (docs *Docs) Server(ctx context.Context, args Optional[[]string]) *Service {
	_args := args.GetOr(nil)

	service := dag.Hugo(docs.HugoVersion).Server(docs.Directory, HugoServerOpts{
		Container: docs._Container(ctx),
		Args:      _args,
	})

	return service
}
