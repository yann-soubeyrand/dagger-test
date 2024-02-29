package main

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
)

const (
	BaseImageRegistry   string = "registry.access.redhat.com"
	BaseImageRepository string = "ubi9-minimal"
	BaseImageTag        string = "9.3-1552"
	BaseImageDigest     string = "sha256:06d06f15f7b641a78f2512c8817cbecaa1bf549488e273f5ac27ff1654ed33f0"

	UtilityImageRegistry   string = "docker.io"
	UtilityImageRepository string = "busybox"
	UtilityImageTag        string = "1.36.1"
	UtilityImageDigest     string = "sha256:ba76950ac9eaa407512c9d859cea48114eeff8a6f12ebaa5d32ce79d4a017dd8"
)

//go:embed bin/sass
var script string

type Sass struct {
	Version  string
	Platform Platform
}

func New(ctx context.Context, version string, platform Optional[Platform]) (*Sass, error) {
	defaultPlatform, err := dag.DefaultPlatform(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get platform: %w", err)
	}

	sass := &Sass{
		Version:  version,
		Platform: platform.GetOr(defaultPlatform),
	}

	return sass, nil
}

func (sass *Sass) Tarball(ctx context.Context) *Directory {
	platform := strings.Split(string(sass.Platform), "/")

	os := platform[0]
	arch := map[string]string{
		"amd64": "x64",
		"386":   "ia32",
		"arm":   "arm",
		"arm64": "arm64",
	}[platform[1]]

	downloadURL := "https://github.com/sass/dart-sass/releases/download/" + sass.Version

	tarballName := fmt.Sprintf("dart-sass-%s-%s-%s.tar.gz", sass.Version, os, arch)

	tarball := dag.HTTP(downloadURL + "/" + tarballName)

	container := dag.Container().
		From(UtilityImageRegistry+"/"+UtilityImageRepository+":"+UtilityImageTag+"@"+UtilityImageDigest).
		WithWorkdir("/home").
		WithMountedFile("sass.tar.gz", tarball).
		WithEntrypoint([]string{"sh", "-c"}).
		WithExec([]string{"tar --extract --strip-components 1 --file sass.tar.gz"})

	directory := dag.Directory().
		WithFile("dart", container.File("src/dart")).
		WithFile("sass.snapshot", container.File("src/sass.snapshot"))

	return directory
}

func (sass *Sass) Directory(ctx context.Context, prefix Optional[string]) *Directory {
	_prefix := prefix.GetOr("/usr/local")

	directory := dag.Directory().
		WithDirectory(_prefix, dag.Directory().
			WithDirectory("libexec/sass", sass.Tarball(ctx)).
			WithNewFile("bin/sass", script, DirectoryWithNewFileOpts{Permissions: 0o755}),
		)

	return directory
}

func (sass *Sass) Container(ctx context.Context, registry Optional[string], repository Optional[string], tag Optional[string], digest Optional[string], packages Optional[[]string]) *Container {
	_registry := registry.GetOr(BaseImageRegistry)
	_repository := repository.GetOr(BaseImageRepository)
	_tag := tag.GetOr(BaseImageTag)
	_digest := digest.GetOr(BaseImageDigest)

	_packages := packages.GetOr(nil)

	container := dag.Container().
		From(_registry + "/" + _repository + ":" + _tag + "@" + _digest)

	if _packages != nil {
		container = container.
			WithEntrypoint([]string{"sh", "-c"}).
			WithExec([]string{"microdnf install --nodocs --setopt install_weak_deps=0 --assumeyes " + strings.Join(_packages, " ") + " && microdnf clean all"})
	}

	container = container.
		WithWorkdir("/home").
		WithDirectory("/", sass.Directory(ctx, OptEmpty[string]())).
		WithEntrypoint([]string{"sass"}).
		WithoutDefaultArgs()

	return container
}

func (sass *Sass) _Container(ctx context.Context) *Container {
	return sass.Container(ctx, OptEmpty[string](), OptEmpty[string](), OptEmpty[string](), OptEmpty[string](), OptEmpty[[]string]())
}

func (sass *Sass) Shell(ctx context.Context, directory *Directory) *Container {
	shell := sass._Container(ctx).
		WithMountedDirectory(".", directory).
		WithEntrypoint([]string{"bash"})

	return shell
}
