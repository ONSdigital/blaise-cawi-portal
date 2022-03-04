# Blaise CAWI Portal


[![codecov](https://codecov.io/gh/ONSdigital/blaise-cawi-portal/branch/main/graph/badge.svg)](https://codecov.io/gh/ONSdigital/blaise-cawi-portal)
[![CI status](https://github.com/ONSdigital/blaise-cawi-portal/workflows/Test%20and%20coverage/badge.svg)](https://github.com/ONSdigital/blaise-cawi-portal/workflows/Test%20coverage%20report/badge.svg)
[![GitHub pull requests](https://img.shields.io/github/issues-pr-raw/ONSdigital/blaise-cawi-portal.svg)](https://github.com/ONSdigital/blaise-cawi-portal/pulls)
[![Github last commit](https://img.shields.io/github/last-commit/ONSdigital/blaise-cawi-portal.svg)](https://github.com/ONSdigital/blaise-cawi-portal/commits)
[![Github contributors](https://img.shields.io/github/contributors/ONSdigital/blaise-cawi-portal.svg)](https://github.com/ONSdigital/blaise-cawi-portal/graphs/contributors)
[![Total alerts](https://img.shields.io/lgtm/alerts/g/ONSdigital/blaise-cawi-portal.svg?logo=lgtm&logoWidth=18)](https://lgtm.com/projects/g/ONSdigital/blaise-cawi-portal/alerts/)

![portal](./portal.gif)

## Inialising Go

**Note**: This is a one off task per repo, but is being documented for future reference

```sh
go mod init github.com/onsdigital/blaise-cawi-portal
```

## Tests

```sh
go test ./...
```

## Run locally

### Create service account credentials

```sh
gcloud iam service-accounts keys create blaise.json --iam-account ons-blaise-v2-<blaise_env>@appspot.gserviceaccount.com
```

### Run

BUS CLient ID:

Open a tunnel to the rest-api:

```sh
gcloud compute start-iap-tunnel restapi-1 80 --local-host-port=localhost:8001
```

```sh
BLAISE_ENV=<blaise-env>
export BLAISE_ENV
gcloud config set project "ons-blaise-v2-${BLAISE_ENV}"
OAUTH_NAME=$(gcloud alpha iap oauth-brands list --format=json | jq -r '.[] | select(.applicationTitle == "blaise").name')
BUS_CLIENT_ID=$(gcloud alpha iap oauth-clients list "${OAUTH_NAME}" --format=json | jq -r '.[] | select(.displayName == "bus").name' | awk -F/ '{print $NF}')
export BUS_CLIENT_ID
```

```sh
BLAISE_REST_API=http://localhost:8001 \
DEV_MODE=true \
GOOGLE_APPLICATION_CREDENTIALS=blaise.json \
BUS_CLIENT_ID=${BUS_CLIENT_ID} \
BUS_URL="https://${BLAISE_ENV}-bus.social-surveys.gcp.onsdigital.uk" \
CATI_URL="https://${BLAISE_ENV}-cati.social-surveys.gcp.onsdigital.uk" \
JWT_SECRET=WFyra7pl8F2M0NuCaPug5pMzzQ073yPU \
SESSION_SECRET=9796ruc2baWXNXixtiqdOcWTSsxltdbm91W7OSJkVIqOtTDhkzVSJAX12VR28B6O \
ENCRYPTION_SECRET=9n5cJirZ4s7jr98XUB0O6XXGxjSh6s8a \
PORT=8080 \
go run main.go
```

*Note*: All of the secrets above are randomly generated examples, these are fine to use when running locally but new
ones should be generated for any deployed environments. This is done at deploy time using terraform.
