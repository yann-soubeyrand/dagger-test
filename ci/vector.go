package main

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const (
	helmChartDependencyName string = "vector"
)

type VectorTarget struct {
	HelmChartPath string
}

func (vector *VectorTarget) UpdateContainerImage(ctx context.Context, registry string, repository string, username string, password *Secret, geoipLicenseKey *Secret) (*Directory, error) {
	helmChart, err := loader.Load(vector.HelmChartPath)

	if err != nil {
		return nil, fmt.Errorf("failed to load Helm chart: %s", err)
	}

	helmChartDependencies := helmChart.Dependencies()

	if len(helmChartDependencies) != 1 || helmChartDependencies[0].Name() != helmChartDependencyName {
		return nil, fmt.Errorf("failed to get Helm chart dependencies")
	}

	helmChart = helmChartDependencies[0]

	var image map[string]any

	if _image := helmChart.Values["image"]; _image != nil {
		image = _image.(map[string]any)
	} else {
		return nil, fmt.Errorf("failed to get image from Helm chart: %s", err)
	}

	var sourceRegistry string
	var sourceRepository string
	var sourceTag string

	if _sourceRegistry := image["registry"]; _sourceRegistry != nil && _sourceRegistry.(string) != "" {
		sourceRegistry = _sourceRegistry.(string)
	} else {
		sourceRegistry = "docker.io"
	}

	if _sourceRepository := image["repository"]; _sourceRepository != nil && _sourceRepository.(string) != "" {
		sourceRepository = _sourceRepository.(string)
	} else {
		return nil, fmt.Errorf("failed to get image repository from Helm chart")
	}

	if _sourceTag := image["tag"]; _sourceTag != nil && _sourceTag.(string) != "" {
		sourceTag = _sourceTag.(string)
	} else if appVersion := helmChart.AppVersion(); appVersion != "" {
		sourceTag = appVersion
	} else {
		return nil, fmt.Errorf("failed to get image tag from Helm chart")
	}

	now := time.Now()

	tagSuffix := fmt.Sprintf("geoip-%d.%02d.%02d", now.Year(), now.Month(), now.Day())

	tag := sourceTag + "_" + tagSuffix

	geoip := dag.Geoip(geoipLicenseKey)

	container := dag.Container().
		From(sourceRegistry+"/"+sourceRepository+":"+sourceTag).
		WithDirectory("/", geoip.Directory([]string{"GeoLite2-City"})).
		WithRegistryAuth(registry, username, password)

	if _, err := container.Publish(ctx, registry+"/"+repository+":"+tag); err != nil {
		return nil, fmt.Errorf("failed to publish container image")
	}

	var helmChartValuesTemplate []byte

	for _, file := range helmChart.Parent().Files {
		if file.Name == "values.tpl.yaml" {
			helmChartValuesTemplate = file.Data
		}
	}

	if helmChartValuesTemplate == nil {
		return nil, fmt.Errorf("failed to get values template from Helm chart")
	}

	tpl, err := template.New("values.yaml").Funcs(sprig.HermeticTxtFuncMap()).Parse(string(helmChartValuesTemplate))

	if err != nil {
		return nil, fmt.Errorf("failed to parse values template from Helm chart: %s", err)
	}

	buf := new(bytes.Buffer)

	err = tpl.Option("missingkey=error").Execute(buf, map[string]any{
		"image": map[string]any{
			"repository": registry + "/" + repository,
			"tag":        tag,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute values template from Helm chart: %s", err)
	}

	directory := dag.Directory().
		WithNewFile("values.yaml", buf.String())

	return directory, nil
}
