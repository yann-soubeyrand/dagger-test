package main

import (
	"context"
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

	CacheDir   string = "/var/cache/hugo"
	goCacheDir string = "/var/cache/go"
)

type Hugo struct {
	Version     string
	SassVersion string
	Platform    Platform
}

func New(ctx context.Context, version string, sassVersion Optional[string], platform Optional[Platform]) (*Hugo, error) {
	defaultPlatform, err := dag.DefaultPlatform(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get platform: %w", err)
	}

	hugo := &Hugo{
		Version:     version,
		SassVersion: sassVersion.GetOr(""),
		Platform:    platform.GetOr(defaultPlatform),
	}

	return hugo, nil
}

func (hugo *Hugo) File(ctx context.Context) *File {
	platform := strings.Split(string(hugo.Platform), "/")

	os := platform[0]
	arch := platform[1]

	downloadURL := "https://github.com/gohugoio/hugo/releases/download/v" + hugo.Version

	tarballName := fmt.Sprintf("hugo_extended_%s_%s-%s.tar.gz", hugo.Version, os, arch)
	checksumsName := fmt.Sprintf("hugo_%s_checksums.txt", hugo.Version)

	tarball := dag.HTTP(downloadURL + "/" + tarballName)
	checksums := dag.HTTP(downloadURL + "/" + checksumsName)

	container := dag.Container().
		From(UtilityImageRegistry+"/"+UtilityImageRepository+":"+UtilityImageTag+"@"+UtilityImageDigest).
		WithWorkdir("/home").
		WithMountedFile(tarballName, tarball).
		WithMountedFile(checksumsName, checksums).
		WithEntrypoint([]string{"sh", "-c"}).
		WithExec([]string{"grep -w " + tarballName + " " + checksumsName + " | sha256sum -c"}).
		WithExec([]string{"tar --extract --file " + tarballName})

	file := container.File("hugo")

	return file
}

func (hugo *Hugo) Directory(ctx context.Context, prefix Optional[string]) *Directory {
	_prefix := prefix.GetOr("/usr/local")

	directory := dag.Directory().
		WithDirectory(_prefix, dag.Directory().
			WithFile("bin/hugo", hugo.File(ctx)),
		)

	if hugo.SassVersion != "" {
		sass := dag.Sass(hugo.SassVersion, SassOpts{
			Platform: string(hugo.Platform),
		})

		directory = directory.
			WithDirectory("/", sass.Directory(SassDirectoryOpts{Prefix: _prefix}))
	}

	return directory
}

func entrypoint(command string) []string {
	return []string{"sh", "-c", "if [ -e 'package.json' ]; then npm ci; fi && exec " + command + " \"$@\"", command}
}

func (hugo *Hugo) Container(ctx context.Context, registry Optional[string], repository Optional[string], tag Optional[string], digest Optional[string], packages Optional[[]string]) *Container {
	_registry := registry.GetOr(BaseImageRegistry)
	_repository := repository.GetOr(BaseImageRepository)
	_tag := tag.GetOr(BaseImageTag)
	_digest := digest.GetOr(BaseImageDigest)

	_packages := append(
		[]string{
			"golang-bin",
			"git",
			"npm",
		},
		packages.GetOr(nil)...,
	)

	container := dag.Container().
		From(_registry+"/"+_repository+":"+_tag+"@"+_digest).
		WithEntrypoint([]string{"sh", "-c"}).
		WithExec([]string{"microdnf module enable nodejs:20 --assumeyes && microdnf install --nodocs --setopt install_weak_deps=0 --assumeyes " + strings.Join(_packages, " ") + " && microdnf clean all"}).
		WithWorkdir("/home").
		WithDirectory("/", hugo.Directory(ctx, OptEmpty[string]())).
		WithMountedCache(CacheDir, dag.CacheVolume("hugo")).
		WithMountedCache(goCacheDir, dag.CacheVolume("go")).
		WithEnvVariable("HUGO_CACHEDIR", CacheDir).
		WithEnvVariable("GOPATH", goCacheDir).
		WithEnvVariable("GOCACHE", goCacheDir+"/build").
		WithEntrypoint(entrypoint("hugo")).
		WithoutDefaultArgs()

	return container
}

func (hugo *Hugo) _Container(ctx context.Context) *Container {
	return hugo.Container(ctx, OptEmpty[string](), OptEmpty[string](), OptEmpty[string](), OptEmpty[string](), OptEmpty[[]string]())
}

func (hugo *Hugo) Shell(ctx context.Context, directory *Directory) *Container {
	shell := hugo._Container(ctx).
		WithMountedDirectory(".", directory).
		WithEntrypoint(entrypoint("bash"))

	return shell
}

func (hugo *Hugo) Build(ctx context.Context, container Optional[*Container], directory *Directory, baseURL string, args Optional[[]string]) *Directory {
	_args := append([]string{"--baseURL", baseURL}, args.GetOr(nil)...)

	output := container.GetOr(hugo._Container(ctx)).
		WithMountedDirectory(".", directory).
		WithExec(_args).
		Directory("public")

	return output
}

func (hugo *Hugo) Server(ctx context.Context, container Optional[*Container], directory *Directory, args Optional[[]string]) *Service {
	_args := append([]string{"server", "--bind", "0.0.0.0"}, args.GetOr(nil)...)

	service := container.GetOr(hugo._Container(ctx)).
		WithMountedDirectory(".", directory).
		WithExec(_args).
		WithExposedPort(1313).
		AsService()

	return service
}
