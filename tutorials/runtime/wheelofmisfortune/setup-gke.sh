#!/bin/bash
set -efo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

CLUSTER_NAME=$1
if [ -z "${CLUSTER_NAME}" ]; then
    echo "cluster name is required as the first parameter!"
fi

ZONE="us-central1-a"
PROJECT_ID=$(gcloud config get project)

# Do nothing if cluster already exists.
if gcloud container clusters list --filter="name: ${CLUSTER_NAME}" 2>&1 | grep -q "^${CLUSTER_NAME} "
then
  echo "WARN: Cluster ${CLUSTER_NAME} already exists, skipping creation"
  gcloud container clusters get-credentials ${CLUSTER_NAME} --zone ${ZONE} --project ${PROJECT_ID}
else
  # Start a new one-node cluster.
  # https://cloud.google.com/sdk/gcloud/reference/container/clusters/create
  # n2-standard-4 -- 4 vCPUs 16 GB
  gcloud container clusters create ${CLUSTER_NAME} \
    --project=${PROJECT_ID} \
    --location=${ZONE} \
    --workload-pool=${PROJECT_ID}.svc.id.goog \
    --release-channel=rapid \
    --num-nodes=1 \
    --machine-type="n2-standard-4"
fi

CLUSTER_API_URL=$(kubectl config view --minify -o jsonpath="{.clusters[?(@.name == \"kind-${CLUSTER_NAME}\")].cluster.server}")
echo "## Cluster is now running, kubectl should point to the new cluster at ${CLUSTER_API_URL}"
kubectl cluster-info

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./brokenapp
CGO_ENABLED=0 go build -o ./brokenapp-mac

# Build image.
docker buildx build --platform linux/amd64 --tag gcr.io/gpe-test-1/bwplotka-brokenapp:latest --load .

# Push image.
docker push gcr.io/gpe-test-1/bwplotka-brokenapp:latest
kubectl delete deployment brokenapp
# Deploy.
kubectl apply -f brokenapp.yaml
