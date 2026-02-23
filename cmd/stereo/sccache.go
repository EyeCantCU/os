package main

import (
	"chainguard.dev/melange/pkg/config"
)

const CgSccacheConfigRepo = "https://apk.cgr.dev/chainguard-private"

func isSccacheEnablePipeline(uses string) bool {
	return uses == "sccache/enable"
}

func sccacheMelangeOpts(cfg *config.Configuration, subcmd string) ([]string, error) {
	pipelines, err := parseUsedPipelines(cfg, subcmd)
	if err != nil {
		return nil, err
	}

	for _, pipeline := range pipelines {
		if findPipeline(pipeline, isSccacheEnablePipeline) {
			return []string{
				"--repository-append", CgSccacheConfigRepo,
				"--package-append", "cg-sccache-config",
				"--env-file", "../os/sccache.env",
			}, nil
		}
	}

	return []string{}, nil
}
