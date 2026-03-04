# pip-check Test Plan

## Objective

Add `test/tw/pip-check` to all Python packages that build with
`py/pip-build-install` but do not yet test with it. This validates that all
runtime Python dependencies declared in a package are actually satisfied.

---

## What pip-check Does

`pip check` verifies that installed packages have all their declared
dependencies present. It catches:

- Missing runtime dependencies (packages referenced in `install_requires` but
  not included in `dependencies.runtime`)
- Incompatible version constraints between installed packages
- Transitive dependency gaps

A pip-check failure means the package is broken from pip's perspective —
imports may fail or behavior may be unexpected at runtime.

---

## Process

### Step 1: Add pip-check to the subpackage test pipeline

In the versioned subpackage range, add `test/tw/pip-check` as the **first**
step in the test pipeline, before any `python/import` or functional tests:

```yaml
    test:
      pipeline:
        - uses: test/tw/pip-check
        - uses: python/import
          with:
            python: python${{range.key}}
            import: ${{vars.import}}
```

No `with:` parameters are needed in the typical case. The pipeline
automatically detects the installed Python version.

### Step 2: Run the test

```bash
make test/<package-name>
```

### Step 3: Interpret results

**Pass:** `No broken requirements found.` — no further action needed.

**Fail:** pip-check prints lines like:

```
<package> <version> requires <dep>, which is not installed.
<package> <version> has requirement <dep>>=<ver>, but you have <dep> <ver>.
```

### Step 4: Remediate failures

For each missing or incompatible dependency reported:

1. Search for the package in the Wolfi / enterprise repos:

   ```bash
   grep -r "name: py3-<dep>" os/ enterprise-packages/ extra-packages/
   ```

2. If the package exists, add it to the `dependencies.runtime` list in the
   correct version-range form:

   ```yaml
   - py${{range.key}}-<dep>
   ```

3. If the package does not exist, it may need to be created first (separate
   work item).

4. For version incompatibilities, check whether the installed version satisfies
   the constraint and update the runtime dep pin if needed.

5. Re-run `make test/<package-name>` after each fix.

### Step 5: Rebuild after fixing deps

After adding missing runtime dependencies, you **must** rebuild the package
before retesting. The test environment installs from the local packages index,
so the old APK (without the new dep declarations) will be used otherwise:

```bash
make package/<package-name>
make test/<package-name>
```

### Step 6: Lint and commit

```bash
./lint.sh <package>.yaml
```

Commit message format: `py3-<package>: add pip-check, fix missing runtime deps`

---

## Notes

- pip-check runs per Python version (3.10, 3.11, 3.12, 3.13). A failure in one
  version may not affect others if the dep is version-gated.
- Some packages use `options: no-depends: true` (e.g. virtual environments,
  bundled deps). These are intentionally exempt from pip-check.
- The `py3-supported-*` meta subpackages do not need pip-check; the versioned
  subpackages cover the actual installs.
- Library dependencies auto-detected by melange (e.g. `so:libssl.so.3`) do not
  appear in pip-check output — only Python package deps are checked.

---

## Results

### Batch 1 (2026-03-04)

All 5 passed with no missing dependencies.

| Package | Directory | pip-check result |
|---|---|---|
| py3-flask | os/ | PASS |
| py3-gitpython | enterprise-packages/ | PASS |
| py3-pydantic | os/ | PASS |
| py3-requests | os/ | PASS |
| py3-structlog | enterprise-packages/ | PASS |

**Conclusion:** These well-maintained packages had complete runtime dep lists.
The earlier celery failure (missing `click-plugins`, `exceptiongroup`) appears
to be an exception rather than the rule for simpler packages.

### Batch 2 (2026-03-04)

All 5 passed with no missing dependencies.

| Package | Directory | pip-check result |
|---|---|---|
| py3-kombu | enterprise-packages/ | PASS |
| py3-langchain-core | enterprise-packages/ | PASS |
| py3-marshmallow | enterprise-packages/ | PASS |
| py3-python-dotenv | enterprise-packages/ | PASS |
| py3-watchdog | enterprise-packages/ | PASS |

**Running total: 10/10 pass. celery remains the only known failure so far.**

### Batch 3 (2026-03-04) — complex packages

3 passed, 2 failed. Failing packages were reverted and not committed.

| Package | Directory | pip-check result | Notes |
|---|---|---|---|
| py3-ansible-core | os/ | PASS | — |
| py3-apache-beam | os/ | FAIL | See below |
| py3-boto3 | os/ | PASS | — |
| py3-kubernetes | os/ | PASS | — |
| py3-poetry | os/ | FAIL | See below |

#### py3-apache-beam failures

Missing runtime deps:
- `envoy-data-plane`
- `jsonpickle`
- `pyarrow-hotfix`
- `sortedcontainers`

Version constraint violations (Wolfi packages newer than beam's upper bounds):
- `grpcio` — beam requires `<1.66.0`, Wolfi has `1.78.1`
- `httplib2` — beam requires `<0.23.0`, Wolfi has `0.31.2`
- `objsize` — beam requires `<0.8.0`, Wolfi has `0.8.0`
- `protobuf` — beam requires `<7.0.0.dev0`, Wolfi has `7.34.0`
- `pyarrow` — beam requires `<19.0.0`, Wolfi has `23.0.1`
- `proto-plus` — also affected by protobuf version

This is a stale upper-bound issue. apache-beam has not updated its
version constraints to match current Wolfi packages. Needs upstream
attention or pinned older versions.

#### py3-poetry failures

Missing runtime dep:
- `fastjsonschema`

Version conflict:
- `trove-classifiers` — poetry requires `>=2022.5.19`, but Wolfi
  has version `1980.1.1.0` (epoch-style versioning that sorts lower
  than the constraint). This is a packaging/version scheme mismatch.

Noise (build dep leaking into test env, not a real runtime issue):
- `scikit-build` complaining about missing `distro` and `wheel`

**Running total: 13/15 pass (87%). 2 failures both in os/, both complex packages with version constraint issues.**

### Batch 4 (2026-03-04) — complex packages, second round

2 passed, 3 failed. Failing packages were reverted and not committed.

| Package | Directory | pip-check result | Notes |
|---|---|---|---|
| py3-dask | os/ | FAIL | See below |
| py3-grpcio-tools | os/ | FAIL | See below |
| py3-httpx | os/ | PASS | — |
| py3-langchain | enterprise-packages/ | FAIL | See below |
| py3-openai | os/ | PASS | — |

#### py3-dask failures

Version constraint violation:
- `toolz` — dask requires `>=0.12.0`, but installed version is `0.0.0`
  (Wolfi's py3-toolz uses epoch-style versioning that sorts as `0.0.0`)

#### py3-langchain failures

Missing runtime deps:
- `langgraph` — not declared in runtime deps and not yet packaged in the
  repo; must be packaged before this can be fixed
- `greenlet` — missing transitive dep via sqlalchemy (py3-greenlet exists
  but not worth adding until langgraph is resolved)

#### py3-grpcio-tools failures

Missing runtime deps:
- `typing-extensions` — required by `grpcio` but not declared
- `setuptools` — required by `grpcio-tools` but not declared

Version constraint violation:
- `protobuf` — grpcio-tools requires `<7.0.0`, Wolfi has `7.34.0`
  (same recurring protobuf version issue as apache-beam)

**Running total: 15/20 pass (75%). protobuf upper-bound constraints are a recurring theme across grpcio-tools and apache-beam.**

### Batch 4 continued — filling to 6 for branch

Added 4 simpler packages to round out the branch to 6 passing packages.
All passed.

| Package | Directory | pip-check result |
|---|---|---|
| py3-click | os/ | PASS |
| py3-rich | os/ | PASS |
| py3-tenacity | os/ | PASS |
| py3-tqdm | os/ | PASS |

**Running total: 19/24 pass (79%).**

### Batch 5 (2026-03-04) — complex packages, third round

6 tested, 4 failed, 2 passed initially. 4 additional simpler packages added to fill branch to 6 passing.

#### First wave (likely-to-fail candidates)

| Package | Directory | pip-check result | Notes |
|---|---|---|---|
| py3-grpcio-status | os/ | FAIL | See below |
| py3-distributed | os/ | FAIL | See below |
| py3-tensorflow-metadata | os/ | FAIL | See below |
| py3-google-api-core | os/ | FAIL | See below |
| py3-aiohttp | os/ | PASS | — |
| py3-botocore | os/ | PASS | — |

#### py3-grpcio-status failures

- Missing runtime dep: `typing-extensions` (required by grpcio)
- Version violation: `googleapis-common-protos` requires `protobuf<6.0.0.dev0`, Wolfi has `7.34.0`
- Version violation: `grpcio-status` requires `protobuf<7.0.0`, Wolfi has `7.34.0`

Same recurring protobuf upper-bound issue as grpcio-tools, apache-beam.

#### py3-distributed failures

- Version violation: `dask` and `distributed` both require `toolz>=0.12.0`, but Wolfi's `py3-toolz` is `0.0.0`

Same toolz epoch-versioning issue as py3-dask.

#### py3-tensorflow-metadata failures

- Version violation: `googleapis-common-protos` requires `protobuf<6.0.0.dev0`, Wolfi has `7.34.0`
- Version violation: `tensorflow-metadata` requires `protobuf<=6.32` (for Python <3.11), Wolfi has `7.34.0`

Same recurring protobuf upper-bound issue.

#### py3-google-api-core failures

- Missing runtime dep: `proto-plus` (not declared in runtime deps)
- Version violation: `google-api-core` requires `protobuf<7.0.0`, Wolfi has `7.34.0`
- Version violation: `googleapis-common-protos` requires `protobuf<6.0.0.dev0`, Wolfi has `7.34.0`

Same recurring protobuf upper-bound issue. Also missing `proto-plus`.

#### Second wave (fill to 6)

| Package | Directory | pip-check result | Notes |
|---|---|---|---|
| py3-sqlalchemy | os/ | FAIL→PASS | Missing `greenlet`; added to runtime deps, rebuilt |
| py3-dnspython | os/ | PASS | — |
| py3-pyyaml | os/ | PASS | — |
| py3-urllib3 | os/ | PASS | — |

**py3-sqlalchemy fix:** `sqlalchemy` requires `greenlet` at runtime but it was not declared. Added `py${{range.key}}-greenlet` to runtime deps and rebuilt.

**Running total: 25/36 pass (69%). protobuf upper-bound and toolz epoch-versioning remain the two dominant systemic failure patterns.**

---

## Working List

### enterprise-packages/

- aws-cfn-bootstrap.yaml
- aws-cli-1.yaml
- bcc.yaml
- cassandra-fips-5.0.yaml
- grype-db.yaml
- kernel-hardening-checker.yaml
- libpwquality.yaml
- py3.13-scanner-test-libraries.yaml
- py3.9-docutils.yaml
- py3.9-installer.yaml
- py3.9-numpy.yaml
- py3.9-pathspec.yaml
- py3.9-pip.yaml
- py3.9-pluggy.yaml
- py3.9-pyyaml.yaml
- py3.9-trove-classifiers.yaml
- py3-apache-tvm-ffi.yaml
- py3-cbor2.yaml
- py3-cfgv.yaml
- py3-channels-redis.yaml
- py3-colored.yaml
- py3-cpuinfo.yaml
- py3-cron-converter.yaml
- py3-cvss.yaml
- py3-dacite.yaml
- py3-dataclasses-json.yaml
- py3-dataclass-wizard.yaml
- py3-diskcache.yaml
- py3-django-countries.yaml
- py3-django-cte.yaml
- py3-django-filter.yaml
- py3-django-guardian.yaml
- py3-django-model-utils.yaml
- py3-django-pgactivity.yaml
- py3-django-pglock.yaml
- py3-django-pgtrigger.yaml
- py3-django-prometheus.yaml
- py3-django-redis.yaml
- py3-dunamai.yaml
- py3-gitdb.yaml
- py3-gitpython.yaml
- py3-google-cloud-pubsub.yaml
- py3-h5py.yaml
- py3-hatch-build-scripts.yaml
- py3-huggingface-hub-0.36.yaml
- py3-identify.yaml
- py3-keras-applications.yaml
- py3-keras-preprocessing.yaml
- py3-keras.yaml
- py3-kombu.yaml
- py3-langchain-core.yaml
- py3-langchain-text-splitters.yaml
- py3-langchain.yaml
- py3-langsmith.yaml
- py3-llguidance.yaml
- py3-llvmlite.yaml
- py3-marshmallow.yaml
- py3-mergedeep.yaml
- py3-ml-dtypes.yaml
- py3-mpi4py.yaml
- py3-nodeenv.yaml
- py3-ntia-conformance-checker.yaml
- py3-omitempty.yaml
- py3-openvino-telemetry.yaml
- py3-opt-einsum.yaml
- py3-oras-py.yaml
- py3-packageurl-python.yaml
- py3-partial-json-parser.yaml
- py3-poetry-dynamic-versioning.yaml
- py3-pre-commit.yaml
- py3-prometheus-fastapi-instrumentator.yaml
- py3-pydantic-extra-types.yaml
- py3-pygobject.yaml
- py3-pymilvus.yaml
- py3-pytest-mock.yaml
- py3-python-dotenv.yaml
- py3-python-multipart.yaml
- py3-python-socketio.yaml
- py3-pyuefivars.yaml
- py3-pyyaml-include.yaml
- py3-rancher-client-python.yaml
- py3-shacl2code.yaml
- py3-smmap.yaml
- py3-spdx-python-model.yaml
- py3-spello.yaml
- py3-structlog.yaml
- py3-systemd.yaml
- py3-ulid-py.yaml
- py3-untokenize.yaml
- py3-uv-dynamic-versioning.yaml
- py3-uvloop.yaml
- py3-virt-firmware.yaml
- py3-watchdog.yaml
- py3-watchfiles.yaml
- py3-xsdata.yaml
- py3-xxhash.yaml
- py3-yardstick.yaml
- vunnel.yaml

### os/

- apache-arrow.yaml
- cassandra-5.0.yaml
- conda-build.yaml
- conda.yaml
- cython-0.yaml
- flatbuffers.yaml
- gi-docgen.yaml
- grpc-1.67.yaml
- grpc-1.68.yaml
- grpc-1.69.yaml
- grpc-1.70.yaml
- grpc-1.71.yaml
- grpc-1.72.yaml
- grpc-1.73.yaml
- grpc-1.74.yaml
- grpc-1.75.yaml
- grpc-1.76.yaml
- grpc-1.78.yaml
- i2c-tools.yaml
- libmamba.yaml
- meson.yaml
- patroni.yaml
- pipx.yaml
- py3-absl-py.yaml
- py3-agate.yaml
- py3-aioboto3.yaml
- py3-aiobotocore.yaml
- py3-aiodataloader.yaml
- py3-aiofiles.yaml
- py3-aiohappyeyeballs.yaml
- py3-aiohttp.yaml
- py3-aioitertools.yaml
- py3-aiosignal.yaml
- py3-aiostream.yaml
- py3-alabaster.yaml
- py3-alembic.yaml
- py3-altair.yaml
- py3-amazon-q-developer-jupyterlab-ext.yaml
- py3-annotated-types.yaml
- py3-ansible-core.yaml
- py3-ansible-runner-http.yaml
- py3-ansible-runner.yaml
- py3-antlr4-python3-runtime.yaml
- py3-anyio.yaml
- py3-apache-beam.yaml
- py3-appdirs.yaml
- py3-appnope.yaml
- py3-archspec.yaml
- py3-argcomplete.yaml
- py3-argon2-cffi-bindings.yaml
- py3-argon2-cffi.yaml
- py3-asgiref.yaml
- py3-asn1crypto.yaml
- py3-astroid.yaml
- py3-asttokens.yaml
- py3-async-generator.yaml
- py3-async-lru.yaml
- py3-asyncssh.yaml
- py3-async-timeout.yaml
- py3-attrs.yaml
- py3-auditwheel.yaml
- py3-autocommand.yaml
- py3-avro-python3.yaml
- py3-awscrt.yaml
- py3-awslambdaric.yaml
- py3-azure-core.yaml
- py3-azure-identity.yaml
- py3-azure-storage-blob.yaml
- py3-babel.yaml
- py3-backcall.yaml
- py3-backoff.yaml
- py3-backports.tarfile.yaml
- py3-bcrypt-3.2.yaml
- py3-bcrypt.yaml
- py3-beartype.yaml
- py3-beautifulsoup4.yaml
- py3-beniget.yaml
- py3-blake3.yaml
- py3-bleach.yaml
- py3-blinker.yaml
- py3-bokeh.yaml
- py3-boltons.yaml
- py3-boolean.py.yaml
- py3-boto3.yaml
- py3-botocore.yaml
- py3-bracex.yaml
- py3-breezy.yaml
- py3-build.yaml
- py3-cachecontrol.yaml
- py3-cached-property.yaml
- py3-cachetools.yaml
- py3-cairo.yaml
- py3-calver.yaml
- py3-canonicaljson.yaml
- py3-cassandra-driver.yaml
- py3-certifi.yaml
- py3-certipy.yaml
- py3-cffi.yaml
- py3-changelog-chug.yaml
- py3-chardet.yaml
- py3-charset-normalizer.yaml
- py3-cheroot.yaml
- py3-cherrypy.yaml
- py3-cleo.yaml
- py3-click-aliases.yaml
- py3-click-default-group.yaml
- py3-click-option-group.yaml
- py3-click.yaml
- py3-cli-helpers.yaml
- py3-cloudpickle.yaml
- py3-cmaes.yaml
- py3-codeowners.yaml
- py3-codespell.yaml
- py3-colorama.yaml
- py3-coloredlogs.yaml
- py3-colorlog.yaml
- py3-commonmark.yaml
- py3-comm.yaml
- py3-conda-index.yaml
- py3-conda-libmamba-solver.yaml
- py3-conda-package-handling.yaml
- py3-conda-package-streaming.yaml
- py3-condense-json.yaml
- py3-configargparse.yaml
- py3-configobj.yaml
- py3-contextlib2.yaml
- py3-contourpy.yaml
- py3-cppy.yaml
- py3-crashtest.yaml
- py3-crcmod.yaml
- py3-cxxfilt.yaml
- py3-cycler.yaml
- py3-cython.yaml
- py3-dask.yaml
- py3-datadog.yaml
- py3-dbus-python.yaml
- py3-debugpy.yaml
- py3-decorator.yaml
- py3-deepmerge.yaml
- py3-defusedxml.yaml
- py3-deprecated.yaml
- py3-deprecation.yaml
- py3-diffoscope.yaml
- py3-dill.yaml
- py3-distlib.yaml
- py3-distributed.yaml
- py3-distro.yaml
- py3-django.yaml
- py3-dnspython.yaml
- py3-docker-squash.yaml
- py3-docker.yaml
- py3-docopt.yaml
- py3-docutils.yaml
- py3-dulwich.yaml
- py3-durationpy.yaml
- py3-editables.yaml
- py3-elfdeps.yaml
- py3-entrypoints.yaml
- py3-envsubst.yaml
- py3-escapism.yaml
- py3-etcd.yaml
- py3-evalidate.yaml
- py3-exceptiongroup.yaml
- py3-execnet.yaml
- py3-executing.yaml
- py3-expandvars.yaml
- py3-extras.yaml
- py3-fabric.yaml
- py3-face.yaml
- py3-faiss-cpu.yaml
- py3-fastavro.yaml
- py3-fastbencode.yaml
- py3-fasteners.yaml
- py3-fastjsonschema.yaml
- py3-filelock.yaml
- py3-findpython.yaml
- py3-flask-cors.yaml
- py3-flask-opentracing.yaml
- py3-flask.yaml
- py3-flit-scm.yaml
- py3-fonttools.yaml
- py3-forestci.yaml
- py3-fqdn.yaml
- py3-fromager.yaml
- py3-frozendict.yaml
- py3-frozenlist.yaml
- py3-fsspec.yaml
- py3-future.yaml
- py3-gast.yaml
- py3-gcloud-aio-auth.yaml
- py3-gcloud-aio-storage.yaml
- py3-gcovr.yaml
- py3-gcsfs.yaml
- py3-geomet.yaml
- py3-gevent.yaml
- py3-gguf.yaml
- py3-git-filter-repo.yaml
- py3-glom.yaml
- py3-glpk.yaml
- py3-google-api-core.yaml
- py3-google-api-python-client.yaml
- py3-googleapis-common-protos.yaml
- py3-google-apitools.yaml
- py3-google-auth-httplib2.yaml
- py3-google-auth-oauthlib.yaml
- py3-google-cloud-bigquery-storage.yaml
- py3-google-cloud-core.yaml
- py3-google-cloud-datastore.yaml
- py3-google-cloud-dlp.yaml
- py3-google-cloud-language.yaml
- py3-google-cloud-recommendations-ai.yaml
- py3-google-cloud-storage.yaml
- py3-google-cloud-videointelligence.yaml
- py3-google-cloud-vision.yaml
- py3-google-crc32c.yaml
- py3-google-pasta.yaml
- py3-google-resumable-media.yaml
- py3-gpep517.yaml
- py3-greenlet.yaml
- py3-grpc-google-iam-v1.yaml
- py3-grpc-interceptor.yaml
- py3-grpcio-gcp.yaml
- py3-grpcio-health-checking.yaml
- py3-grpcio-opentracing.yaml
- py3-grpcio-reflection.yaml
- py3-grpcio-status.yaml
- py3-grpcio-tools.yaml
- py3-gunicorn.yaml
- py3-gyp-next.yaml
- py3-h11.yaml
- py3-hatch-fancy-pypi-readme.yaml
- py3-hatch-jupyter-builder.yaml
- py3-hatch-nodejs-version.yaml
- py3-hatch-requirements-txt.yaml
- py3-hatch-vcs.yaml
- py3-hatch.yaml
- py3-hdfs.yaml
- py3-hologram.yaml
- py3-html5lib.yaml
- py3-httpcore.yaml
- py3-httplib2.yaml
- py3-httpx.yaml
- py3-huggingface-hub.yaml
- py3-humanfriendly.yaml
- py3-hyperlink.yaml
- py3-hyperopt.yaml
- py3-idna.yaml
- py3-imagesize.yaml
- py3-importlib-metadata.yaml
- py3-importlib-resources.yaml
- py3-influxdb-client.yaml
- py3-iniconfig.yaml
- py3-installer.yaml
- py3-invoke.yaml
- py3-ipaddress.yaml
- py3-ipykernel.yaml
- py3-ipython-genutils.yaml
- py3-ipython.yaml
- py3-ipywidgets.yaml
- py3-iso8601.yaml
- py3-isodate.yaml
- py3-isort.yaml
- py3-itables.yaml
- py3-itsdangerous.yaml
- py3-jaeger-client.yaml
- py3-jaraco.classes.yaml
- py3-jaraco.collections.yaml
- py3-jaraco.context.yaml
- py3-jaraco.functools.yaml
- py3-jaraco.text.yaml
- py3-javaproperties.yaml
- py3-jedi.yaml
- py3-jeepney.yaml
- py3-jinja2.yaml
- py3-jiter.yaml
- py3-jmespath.yaml
- py3-joblib.yaml
- py3-json5.yaml
- py3-jsondiff.yaml
- py3-jsonpatch.yaml
- py3-jsonpointer.yaml
- py3-jsonschema-specifications.yaml
- py3-jsonschema.yaml
- py3-jupyter-client.yaml
- py3-jupyter-console.yaml
- py3-jupyter-core.yaml
- py3-jupyter-events.yaml
- py3-jupyterhub-firstuseauthenticator.yaml
- py3-jupyterhub-hmacauthenticator.yaml
- py3-jupyterhub-idle-culler.yaml
- py3-jupyterhub-kubespawner.yaml
- py3-jupyterhub-ldapauthenticator.yaml
- py3-jupyterhub-ltiauthenticator.yaml
- py3-jupyterhub-nativeauthenticator.yaml
- py3-jupyterhub-tmpauthenticator.yaml
- py3-jupyterhub.yaml
- py3-jupyterlab-pygments.yaml
- py3-jupyterlab-server.yaml
- py3-jupyterlab.yaml
- py3-jupyter-lsp.yaml
- py3-jupyter-packaging.yaml
- py3-jupyter-server-fileid.yaml
- py3-jupyter-server-terminals.yaml
- py3-jupyter-server.yaml
- py3-jupyter-telemetry.yaml
- py3-jupyter-ydoc.yaml
- py3-jwcrypto.yaml
- py3-kiwisolver.yaml
- py3-knack.yaml
- py3-kubernetes-asyncio.yaml
- py3-kubernetes.yaml
- py3-ldap3.yaml
- py3-leather.yaml
- py3-legacy-cgi.yaml
- py3-libarchive-c.yaml
- py3-libclang.yaml
- py3-libcst.yaml
- py3-libevdev.yaml
- py3-license-expression.yaml
- py3-llhttp.yaml
- py3-llm.yaml
- py3-locket.yaml
- py3-lockfile.yaml
- py3-logbook.yaml
- py3-logfmter.yaml
- py3-lru-dict.yaml
- py3-lxml.yaml
- py3-lz4.yaml
- py3-magic.yaml
- py3-mailbits.yaml
- py3-mako.yaml
- py3-markdown-it-py.yaml
- py3-markdown.yaml
- py3-markupsafe.yaml
- py3-mashumaro.yaml
- py3-matplotlib-inline.yaml
- py3-matplotlib.yaml
- py3-maturin.yaml
- py3-mdit-plain.yaml
- py3-mdit-py-plugins.yaml
- py3-mdurl.yaml
- py3-merge3.yaml
- py3-meson-python.yaml
- py3-minimal-snowplow-tracker.yaml
- py3-mistune.yaml
- py3-ml-metadata.yaml
- py3-mock.yaml
- py3-more-itertools.yaml
- py3-mpmath.yaml
- py3-msal-extensions.yaml
- py3-msal.yaml
- py3-msgspec.yaml
- py3-multidict.yaml
- py3-mwoauth.yaml
- py3-mypy-extensions.yaml
- py3-namex.yaml
- py3-narwhals.yaml
- py3-nbclient.yaml
- py3-nbconvert.yaml
- py3-nbformat.yaml
- py3-nest-asyncio.yaml
- py3-netifaces.yaml
- py3-networkx.yaml
- py3-nh3.yaml
- py3-nltk.yaml
- py3-notebook-shim.yaml
- py3-notify2.yaml
- py3-nullauthenticator.yaml
- py3-oauth2client.yaml
- py3-oauthenticator.yaml
- py3-oauthlib.yaml
- py3-objsize.yaml
- py3-onetimepass.yaml
- py3-openai.yaml
- py3-opentracing.yaml
- py3-optree.yaml
- py3-optuna.yaml
- py3-ordered-set.yaml
- py3-orjson.yaml
- py3-oscrypto.yaml
- py3-outcome.yaml
- py3-overrides.yaml
- py3-packaging.yaml
- py3-pamela.yaml
- py3-pandas.yaml
- py3-pandocfilters.yaml
- py3-paramiko.yaml
- py3-parsedatetime.yaml
- py3-parso.yaml
- py3-partd.yaml
- py3-pathlib2.yaml
- py3-pathspec.yaml
- py3-path.yaml
- py3-patiencediff.yaml
- py3-pbr.yaml
- py3-pbs_installer.yaml
- py3-pdm-backend.yaml
- py3-pecan.yaml
- py3-peewee.yaml
- py3-pefile.yaml
- py3-pendulum.yaml
- py3-pep517.yaml
- py3-pexpect.yaml
- py3-pgcli.yaml
- py3-pgspecial.yaml
- py3-pickleshare.yaml
- py3-pillow.yaml
- py3-pipenv.yaml
- py3-pip-tools.yaml
- py3-pkgconfig.yaml
- py3-pkginfo.yaml
- py3-pkgutil_resolve_name.yaml
- py3-platformdirs.yaml
- py3-pluggy.yaml
- py3-ply.yaml
- py3-poetry-core.yaml
- py3-poetry.yaml
- py3-portalocker.yaml
- py3-portend.yaml
- py3-prettytable.yaml
- py3-prometheus-client.yaml
- py3-prompt-toolkit.yaml
- py3-propcache.yaml
- py3-protobuf.yaml
- py3-proto-plus.yaml
- py3-psutil.yaml
- py3-psycopg-16.yaml
- py3-psycopg-17.yaml
- py3-psycopg-18.yaml
- py3-psycopg2.yaml
- py3-ptyprocess.yaml
- py3-pure-eval.yaml
- py3-puremagic-1.yaml
- py3-puremagic-2.yaml
- py3-py4j.yaml
- py3-pyaes.yaml
- py3-pyasn1-modules.yaml
- py3-pyasn1.yaml
- py3-pybase64.yaml
- py3-pybind11.yaml
- py3-pycosat.yaml
- py3-pycparser.yaml
- py3-pycrdt-websocket.yaml
- py3-pycrdt.yaml
- py3-pycryptodome.yaml
- py3-pycurl.yaml
- py3-pydantic-core.yaml
- py3-pydantic.yaml
- py3-pydot.yaml
- py3-pyelftools.yaml
- py3-pyfarmhash.yaml
- py3-pygithub.yaml
- py3-pygments.yaml
- py3-pyjwt.yaml
- py3-pykube-ng.yaml
- py3-pymongo.yaml
- py3-pymysql.yaml
- py3-pynacl.yaml
- py3-pyopenssl.yaml
- py3-pyparsing.yaml
- py3-pyperclip.yaml
- py3-pypiserver.yaml
- py3-pypi-simple.yaml
- py3-pyproject_api.yaml
- py3-pyproject-hooks.yaml
- py3-pyproject-metadata.yaml
- py3-pyrfc3339.yaml
- py3-pyrsistent.yaml
- py3-pyserial.yaml
- py3-pystache.yaml
- py3-pysyncobj.yaml
- py3-pytest-timeout.yaml
- py3-pytest-xdist.yaml
- py3-pytest.yaml
- py3-python-crfsuite.yaml
- py3-python-daemon.yaml
- py3-python-discovery.yaml
- py3-python-editor.yaml
- py3-python-gitlab.yaml
- py3-python-json-logger.yaml
- py3-python-lsp-jsonrpc.yaml
- py3-python-pypi-mirror.yaml
- py3-python-slugify.yaml
- py3-python-ulid.yaml
- py3-pythran.yaml
- py3-pytimeparse.yaml
- py3-pytz.yaml
- py3-pywin32-ctypes.yaml
- py3-pyyaml.yaml
- py3-pyzmq.yaml
- py3-rapidfuzz.yaml
- py3-rdflib.yaml
- py3-reactivex.yaml
- py3-recommonmark.yaml
- py3-referencing.yaml
- py3-regex.yaml
- py3-reno.yaml
- py3-repoze.lru.yaml
- py3-requests-oauthlib.yaml
- py3-requests-toolbelt.yaml
- py3-requests-unixsocket.yaml
- py3-requests.yaml
- py3-resolvelib.yaml
- py3-retrying.yaml
- py3-rfc3339-validator.yaml
- py3-rfc3986-validator.yaml
- py3-rich.yaml
- py3-robotframework.yaml
- py3-roman-numerals-py.yaml
- py3-roman.yaml
- py3-rouge-score.yaml
- py3-routes.yaml
- py3-rpds-py.yaml
- py3-rsa.yaml
- py3-ruamel-yaml-clib.yaml
- py3-ruamel-yaml.yaml
- py3-s3transfer.yaml
- py3-sacrebleu.yaml
- py3-scandir.yaml
- py3-scikit-build.yaml
- py3-scikit-learn.yaml
- py3-scipy.yaml
- py3-scp.yaml
- py3-seaborn.yaml
- py3-secretstorage.yaml
- py3-semantic-version.yaml
- py3-semver.yaml
- py3-send2trash.yaml
- py3-setproctitle.yaml
- py3-setuptools-gettext.yaml
- py3-setuptools-git-versioning.yaml
- py3-setuptools-rust.yaml
- py3-setuptools-scm.yaml
- py3-shellingham.yaml
- py3-simplejson.yaml
- py3-six.yaml
- py3-sniffio.yaml
- py3-snowballstemmer.yaml
- py3-sortedcontainers.yaml
- py3-soupsieve.yaml
- py3-spdx-tools.yaml
- py3-sphinx-7.yaml
- py3-sphinxcontrib-applehelp.yaml
- py3-sphinxcontrib-devhelp.yaml
- py3-sphinxcontrib-htmlhelp.yaml
- py3-sphinxcontrib-jquery.yaml
- py3-sphinxcontrib-packages.yaml
- py3-sphinxcontrib-qthelp.yaml
- py3-sphinxcontrib-serializinghtml.yaml
- py3-sphinx-rtd-theme.yaml
- py3-sqlalchemy-cockroachdb.yaml
- py3-sqlalchemy.yaml
- py3-sqlglot.yaml
- py3-sqlite-anyio.yaml
- py3-sqlite-fts4.yaml
- py3-sqlite-migrate.yaml
- py3-sqlite-utils.yaml
- py3-sqlparse.yaml
- py3-sshtunnel.yaml
- py3-stack-data.yaml
- py3-starlette.yaml
- py3-statsd.yaml
- py3-stevedore.yaml
- py3-sympy.yaml
- py3-tabulate.yaml
- py3-tblib.yaml
- py3-tempora.yaml
- py3-tenacity.yaml
- py3-tensorflow-metadata.yaml
- py3-terminado.yaml
- py3-testpath.yaml
- py3-testtools.yaml
- py3-text-unidecode.yaml
- py3-threadloop.yaml
- py3-threadpoolctl.yaml
- py3-tinycss2.yaml
- py3-tinydb.yaml
- py3-tomli-w.yaml
- py3-tomlkit.yaml
- py3-toml.yaml
- py3-toolz.yaml
- py3-tornado.yaml
- py3-tox.yaml
- py3-tqdm.yaml
- py3-traitlets.yaml
- py3-trio.yaml
- py3-trove-classifiers.yaml
- py3-twython.yaml
- py3-typing-extensions.yaml
- py3-typing-inspection.yaml
- py3-typing-inspect.yaml
- py3-typogrify.yaml
- py3-tzdata.yaml
- py3-tzlocal.yaml
- py3-tz.yaml
- py3-udev.yaml
- py3-ujson.yaml
- py3-uritemplate.yaml
- py3-uritools.yaml
- py3-urllib3.yaml
- py3-userpath.yaml
- py3-uuid-utils.yaml
- py3-vcrpy.yaml
- py3-versioneer.yaml
- py3-virtualenv.yaml
- py3-wcmatch.yaml
- py3-wcwidth.yaml
- py3-webcolors.yaml
- py3-webencodings.yaml
- py3-webob.yaml
- py3-websocket-client.yaml
- py3-werkzeug.yaml
- py3-widgetsnbextension.yaml
- py3-wrapt.yaml
- py3-xattr.yaml
- py3-xet-core.yaml
- py3-xmlsec.yaml
- py3-xmltodict.yaml
- py3-xyzservices.yaml
- py3-yamale.yaml
- py3-yarl.yaml
- py3-ydiff.yaml
- py3-zaproxy.yaml
- py3-zict.yaml
- py3-zipp.yaml
- py3-zope.event.yaml
- py3-zope.interface.yaml
- py3-zstandard.yaml
- pylint.yaml
- s3cmd.yaml
- ssh-import-id.yaml
- strongswan.yaml
- suricata-update.yaml
- thrift.yaml
- uv.yaml
- websockify.yaml
- yamllint.yaml
