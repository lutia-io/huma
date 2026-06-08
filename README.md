# Huma

The Huma bird is a legendary creature deeply rooted in Persian and Islamic mythology, representing prosperity, good fortune, and royalty. Known as a mythical bird of paradise, it is believed that whoever is touched by the shadow or sight of the Huma bird is destined for greatness, kingship, or profound happiness.

In Persian and Sufi traditions, the Huma never rests on the ground, perpetually soaring above earthly desires. This symbolizes its divine purity and freedom from worldly attachments. Its presence is often interpreted as an omen signifying the rise of a just and righteous ruler. The Huma bird, therefore, embodies benevolence, divine favor, and spiritual elevation.

<https://en.wikipedia.org/wiki/Huma_bird>

```text
Usage: huma <command> [options]

Commands:
  api      The api service serves the main public api.
```


## Requirements

- Go
- Kubernetes
- Kind
- Helm
- Skaffold


## Quick Start

Follow these steps to set up and run the Huma platform locally.

### 1. Clone the Repository
```bash
mkdir huma-project
cd huma-project
git clone git@github.com:lutia-io/huma.git
cd huma
```

### 2. Configure Environment
Create a new `skaffold.env` file and add the required environment variables. `You can view .env.example` for the sample environment file with variables.

### 3. Launch the Huma Platform
Create a Kind Kubernetes Cluster:

```bash
kind create cluster --config k8s/local-cluster.yaml
```

Set up [Cloud Provider Kind](https://github.com/kubernetes-sigs/cloud-provider-kind):

```bash
go install sigs.k8s.io/cloud-provider-kind@latest
sudo install ~/go/bin/cloud-provider-kind /usr/local/bin
sudo cloud-provider-kind
```

Release the platform in development mode using Skaffold:

```bash
skaffold dev
```


## Testing

Execute all tests:
```
go test ./... -cover
```

Generate a coverage report:
```
go test ./... -coverprofile=coverage.out

# inspect `coverage.out`:
# 1. console inspection
go tool cover -func=coverage.out

# 2. HTML inspection
go tool cover -html=coverage.out
```