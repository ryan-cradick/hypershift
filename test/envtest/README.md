# HyperShift envtest API Validation

This directory contains YAML-driven integration tests that validate HyperShift CRD schemas
(including CEL validation rules) using [envtest](https://book.kubebuilder.io/reference/envtest).

Tests run against multiple Kubernetes and OCP API server versions to catch compatibility issues
across releases.

## How it works

1. Each test suite get its CRD installed and uninstalled before and after running.
2. Test suites are defined in cmd/install/assets/hypershift-operator/tests, as `{stable/techpreview}.{CRDName}.{suiteCase}.testsuite.yaml` files following the
   [openshift/api tests](https://github.com/openshift/api/tree/master/tests) format.
3. Each test case creates a resource from inline YAML and asserts either success or a specific validation error substring.

## Running

```bash
# Run against all supported OCP and vanilla Kubernetes versions
make test-envtest-api-all

# Run only OCP versions (4.17–4.22)
make test-envtest-ocp

# Run only vanilla Kubernetes versions (1.31–1.35)
make test-envtest-kube

# Run against a single version
make test-envtest-ocp ENVTEST_OCP_K8S_VERSIONS="1.34.1"

# These tests also run as part of `make test`
```

### Parallel execution

By default, versions run sequentially. Use `ENVTEST_JOBS` to run multiple versions
in parallel — each version gets its own isolated envtest environment (etcd + kube-apiserver):

| Value | Behaviour |
|-------|-----------|
| `0` (default) | Sequential — one version at a time |
| `N` | Run up to N versions in parallel |
| `MAX` | Run all versions in parallel |

```bash
# Run 3 OCP versions in parallel
make test-envtest-ocp ENVTEST_JOBS=3

# Run all OCP versions in parallel
make test-envtest-ocp ENVTEST_JOBS=MAX

# Run all Kubernetes versions in parallel
make test-envtest-kube ENVTEST_JOBS=MAX

# Works with the combined target too
make test-envtest-api-all ENVTEST_JOBS=MAX
```

## Test format reference

The YAML format is compatible with
[openshift/api tests](https://github.com/openshift/api/tree/master/tests).
See [types.go](./types.go) for the Go type definitions.
