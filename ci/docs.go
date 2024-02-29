package main

import (
	"context"
)

type DocsTarget struct {
	Directory *Directory
}

func (docs *DocsTarget) PublishContainerImage(ctx context.Context, registry string, repository string, username string, password *Secret) (string, error) {
	container := dag.Docs(docs.Directory).Container().
		WithRegistryAuth(registry, username, password)

	return container.Publish(ctx, registry+"/"+repository)
}

func (docs *DocsTarget) Build(ctx context.Context, baseURL Optional[string]) *Directory {
	return dag.Docs(docs.Directory).Build(baseURL.GetOr(""))
}

func (docs *DocsTarget) Server(ctx context.Context) *Service {
	return dag.Docs(docs.Directory).Server()
}
