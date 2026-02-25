package main

import (
	"chainguard.dev/melange/pkg/config"
)

const CgSccacheConfigRepo = EnterprisePackagesRepo

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
				"--repository-append=" + CgSccacheConfigRepo,
				"--env-file=../os/sccache.env",
			}, nil
		}
	}

	return []string{}, nil
}
