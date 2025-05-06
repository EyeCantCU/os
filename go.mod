module chainguard.dev/apkoaas

go 1.24.2

require (
	chainguard.dev/apko v0.27.2
	github.com/anchore/syft v1.23.1
	github.com/chainguard-dev/clog v1.7.0
	github.com/charmbracelet/log v0.4.1
	github.com/google/go-cmp v0.7.0
	github.com/google/go-containerregistry v0.20.4-0.20250225234217-098045d5e61f
	github.com/spf13/cobra v1.9.1
	github.com/wolfi-dev/wolfictl v0.36.6
	gopkg.in/yaml.v3 v3.0.1
)

// https://github.com/anchore/grype/blob/v0.80.1/go.mod#L266-L269
// Pull in a fix for an unpatched CVE. mholt/archiver appears inactive/unmaintained.
replace github.com/mholt/archiver/v3 => github.com/anchore/archiver/v3 v3.5.2

// https://github.com/anchore/grype/blob/v0.80.1/go.mod#L266-L269
// this is a breaking change, so we need to pin the version until glebarez/go-sqlite is updated to use internal/libc
replace modernc.org/sqlite v1.33.0 => modernc.org/sqlite v1.32.0

// Fork of the golang vuln to make private method public.
replace golang.org/x/vuln => github.com/chainguard-dev/golang-vuln v1.1.3

require (
	chainguard.dev/go-grpc-kit v0.17.10 // indirect
	chainguard.dev/sdk v0.1.32 // indirect
	cloud.google.com/go/auth v0.16.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	dario.cat/mergo v1.0.1 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20231105174938-2b5cbb29f3e2 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/CycloneDX/cyclonedx-go v0.9.2 // indirect
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.12.9 // indirect
	github.com/ProtonMail/go-crypto v1.2.0 // indirect
	github.com/STARRY-S/zip v0.2.3 // indirect
	github.com/acobaugh/osrelease v0.1.0 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/anchore/archiver/v3 v3.5.3-0.20241210171143-5b1d8d1c7c51 // indirect
	github.com/anchore/clio v0.0.0-20250408180537-ec8fa27f0d9f // indirect
	github.com/anchore/fangs v0.0.0-20250402135612-96e29e45f3fe // indirect
	github.com/anchore/go-collections v0.0.0-20241211140901-567f400e9a46 // indirect
	github.com/anchore/go-homedir v0.0.0-20250319154043-c29668562e4d // indirect
	github.com/anchore/go-logger v0.0.0-20250318195838-07ae343dd722 // indirect
	github.com/anchore/go-macholibre v0.0.0-20250320151634-807da7ad2331 // indirect
	github.com/anchore/go-struct-converter v0.0.0-20250211213226-cce56d595160 // indirect
	github.com/anchore/go-sync v0.0.0-20250408180515-6585ecec709b // indirect
	github.com/anchore/go-version v1.2.2-0.20210903204242-51efa5b487c4 // indirect
	github.com/anchore/packageurl-go v0.1.1-0.20250220190351-d62adb6e1115 // indirect
	github.com/anchore/stereoscope v0.1.4 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/aquasecurity/go-pep440-version v0.0.1 // indirect
	github.com/aquasecurity/go-version v0.0.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/becheran/wildmatch-go v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitnami/go-version v0.0.0-20250324202741-04b9d491e744 // indirect
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.3.0 // indirect
	github.com/charmbracelet/lipgloss v1.1.1-0.20250319133953-166f707985bc // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be // indirect
	github.com/containerd/cgroups/v3 v3.0.5 // indirect
	github.com/containerd/containerd v1.7.27 // indirect
	github.com/containerd/containerd/api v1.8.0 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/deitch/magic v0.0.0-20240306090643-c67ab88f10cb // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v28.1.1+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.1.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-events v0.0.0-20250114142523-c867878c5e32 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elliotchance/phpserialize v1.4.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/facebookincubator/nvdtools v0.1.5 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/github/go-spdx/v2 v2.3.2 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-git/go-git/v5 v5.16.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-restruct/restruct v1.2.0-alpha // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gohugoio/hashstructure v0.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/licensecheck v0.3.1 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20210315223345-82c243799c99 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl/v2 v2.23.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/kastenhq/goversion v0.0.0-20230811215019-93b2f8823953 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/knqyf263/go-rpmdb v0.1.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/archives v0.1.1 // indirect
	github.com/minio/minlz v1.0.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/nwaples/rardecode/v2 v2.1.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.2.1 // indirect
	github.com/opencontainers/selinux v1.12.0 // indirect
	github.com/package-url/packageurl-go v0.1.3 // indirect
	github.com/pborman/indent v1.2.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rust-secure-code/go-rustaudit v0.0.0-20250226111315-e20ec32e963c // indirect
	github.com/sagikazarmark/locafero v0.9.0 // indirect
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d // indirect
	github.com/sassoftware/go-rpmutils v0.4.0 // indirect
	github.com/scylladb/go-set v1.0.3-0.20200225121959-cc7b2070d91e // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/sorairolake/lzip-go v0.3.7 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spdx/gordf v0.0.0-20250128162952-000978ccd6fb // indirect
	github.com/spdx/tools-golang v0.5.5 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/spf13/viper v1.20.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/sylabs/sif/v2 v2.21.1 // indirect
	github.com/sylabs/squashfs v1.0.6 // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/u-root/u-root v0.14.0 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/vbatts/go-mtree v0.5.4 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vifraa/gopom v1.0.0 // indirect
	github.com/wagoodman/go-partybus v0.0.0-20230516145632-8ccac152c651 // indirect
	github.com/wagoodman/go-progress v0.0.0-20230925121702-07e42b3cdba0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/zclconf/go-cty v1.16.2 // indirect
	go.lsp.dev/uri v0.3.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.step.sm/crypto v0.61.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.29.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/api v0.230.0 // indirect
	google.golang.org/genproto v0.0.0-20250414145226-207652e42e2e // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250425173222-7b384671a197 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250425173222-7b384671a197 // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	sigs.k8s.io/release-utils v0.11.1 // indirect
)
