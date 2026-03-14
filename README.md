# helm-image-finder

Extracts all container images from a Helm chart by rendering it with `helm template` and walking the resulting Kubernetes manifests. Useful for auditing images before deployment, scanning for vulnerabilities, or building image pull lists for air-gapped environments.

## Requirements

- [Go](https://go.dev/) 1.21+
- [Helm](https://helm.sh/) v3 in your `$PATH`

## Installation

```bash
git clone https://github.com/yourorg/helm-image-finder
cd helm-image-finder
go build -o helm-image-finder .
```

Or install directly:

```bash
go install helm-image-finder@latest
```

## Usage

```
helm-image-finder [flags] <release-name> <chart>
```

Flags can appear before or after the positional arguments.

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--output` | `-o` | `table` | Output format: `table`, `json`, `csv`, `list` |
| `--unique` | `-u` | false | Only show unique image+tag combinations (first occurrence wins) |
| `--values` | `-f` | — | Path to a values file; may be repeated |
| `--set` | — | — | Override a chart value (helm `--set` syntax); may be repeated |
| `--namespace` | `-n` | — | Kubernetes namespace to pass to `helm template` |

## Examples

### Basic usage

```bash
helm-image-finder my-release ./my-chart
```

```
RESOURCE                              CHART               CONTAINER        IMAGE                     TAG
------------------------              ---------------     -----------      -------------------------  -------
Deployment/default/my-app             my-app-1.0.0        app              myregistry.io/my-app      v1.2.3
Deployment/default/my-app             my-app-1.0.0        sidecar          prom/prometheus           v2.45.0
StatefulSet/default/my-db             postgresql-12.1.0   db               postgres                  15.2
CronJob/default/my-cleaner            my-app-1.0.0        cleaner          busybox                   1.36
```

The CHART column is sourced from the `helm.sh/chart` label that Helm stamps on every rendered resource. For umbrella charts (charts that pull in subcharts), this identifies which subchart owns each image — each subchart will show its own name and version.

### With values files and overrides

```bash
helm-image-finder my-release ./my-chart \
  -f base-values.yaml \
  -f prod-values.yaml \
  --set image.tag=v2.0.0
```

### Remote chart from a registry

```bash
helm-image-finder my-release oci://registry.io/org/chart --version 1.5.0
```

### Remote chart from a repo

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm-image-finder my-release bitnami/nginx
```

### Unique images only

Useful when multiple containers share an image and you just want the distinct set — for example, to build a pull list.

```bash
helm-image-finder my-release ./my-chart --unique
```

```
RESOURCE                              CONTAINER        IMAGE                     TAG
------------------------              -----------      -------------------------  -------
Deployment/default/my-app             app              myregistry.io/my-app      v1.2.3
Deployment/default/my-app             sidecar          prom/prometheus           v2.45.0
StatefulSet/default/my-db             db               postgres                  15.2
```

### JSON output

```bash
helm-image-finder my-release ./my-chart --output json
```

```json
[
  {
    "kind": "Deployment",
    "namespace": "default",
    "resourceName": "my-app",
    "chartLabel": "my-app-1.0.0",
    "containerName": "app",
    "image": "myregistry.io/my-app",
    "tag": "v1.2.3"
  },
  ...
]
```

### List output (Zarf / air-gapped)

Plain `image:tag` per line — ready to paste directly into a [Zarf](https://zarf.dev/) package `images:` block or any tool that wants a bare image list.

```bash
helm-image-finder my-release ./my-chart --output list --unique
```

```
myregistry.io/my-app:v1.2.3
prom/prometheus:v2.45.0
postgres:15.2
busybox:1.36
```

### CSV output

```bash
helm-image-finder my-release ./my-chart --output csv
```

```
resource,chart,container,image,tag
Deployment/default/my-app,my-app-1.0.0,app,myregistry.io/my-app,v1.2.3
Deployment/default/my-app,my-app-1.0.0,sidecar,prom/prometheus,v2.45.0
StatefulSet/default/my-db,postgresql-12.1.0,db,postgres,15.2
```

### Pipe CSV into other tools

```bash
# Pull all unique images
helm-image-finder my-release ./my-chart --output csv --unique \
  | tail -n +1 \
  | awk -F',' '{ print $3 ":" $4 }' \
  | xargs -I{} docker pull {}

# Find images from a specific registry
helm-image-finder my-release ./my-chart --output csv \
  | grep myregistry.io
```

## How it works

1. Runs `helm template <release> <chart> [flags]` and captures the rendered YAML
2. Decodes each YAML document and reads `kind`, `metadata.name`, and `metadata.namespace` for resource context
3. Recursively walks the full YAML node tree looking for any mapping that contains an `image` key — this catches containers, init containers, sidecars, ephemeral containers, and images nested inside CronJobs, Jobs, or custom resource wrappers without needing to know the shape of every resource type
4. Formats and prints the collected entries

Image references are split into `image` and `tag` components. Tag-less references (e.g. `nginx`) report `latest`. Digest references (e.g. `nginx@sha256:abc123`) report the full digest as the tag.

## Project structure

```
helm-image-finder/
├── main.go      — Config struct, flag parsing, main orchestration
├── helm.go      — shells out to helm template
├── extract.go   — YAML node walking, image extraction, deduplication
└── output.go    — table, JSON, CSV, and list formatters
```
