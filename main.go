package main

import (
	"context"
	"dagger/test/internal/dagger"
)

type Test struct{}

func (*Test) Test(ctx context.Context, secret *dagger.Secret) (string, error) {
	plaintext, err := secret.Plaintext(ctx)

	if err != nil {
		return "", err
	}

	return dag.Container().
		From("docker.io/library/busybox").
		WithSecretVariable("SECRET", secret).
		WithEnvVariable("PLAINTEXT", plaintext).
		WithExec([]string{"sh", "-c", "echo prefix.$SECRET.suffix prefix.$PLAINTEXT.suffix"}).
		Stdout(ctx)
}
