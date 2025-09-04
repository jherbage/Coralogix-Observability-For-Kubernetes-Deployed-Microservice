#!/bin/bash

set -e

# Check if CORALOGIX_APIKEY is set
if [[ -z "${CORALOGIX_APIKEY}" ]]; then
  echo "Error: The environment variable CORALOGIX_APIKEY is not set."
  echo "Please set it using the command: export CORALOGIX_APIKEY=<your-api-key>"
  exit 1
fi

echo "Starting Minikube..."
minikube start

echo "Waiting for Kubernetes to be ready..."
while [[ $(kubectl get nodes -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do
  echo "Kubernetes is not ready yet. Retrying in 5 seconds..."
  sleep 5
done

echo "Kubernetes is ready!"

# Create the 'demo' namespace
NAMESPACE="demo"
if kubectl get namespace "$NAMESPACE" > /dev/null 2>&1; then
  echo "Namespace '$NAMESPACE' already exists."
else
  echo "Creating namespace '$NAMESPACE'..."
  kubectl create namespace "$NAMESPACE"
  echo "Namespace '$NAMESPACE' created successfully."
fi

# Create the 'coralogix-keys' secret
SECRET_NAME="coralogix-keys"
if kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" > /dev/null 2>&1; then
  echo "Secret '$SECRET_NAME' already exists in namespace '$NAMESPACE'."
else
  echo "Creating secret '$SECRET_NAME' in namespace '$NAMESPACE'..."
  kubectl create secret generic "$SECRET_NAME" \
    --from-literal=PRIVATE_KEY="$CORALOGIX_APIKEY" \
    -n "$NAMESPACE"
  echo "Secret '$SECRET_NAME' created successfully."
fi

# Install cert-manager using Helm - needed for opentelemetry operator
helm repo add jetstack https://charts.jetstack.io 2>&1
helm repo update 2>&1

if kubectl get namespace cert-manager > /dev/null 2>&1; then
  echo "Namespace cert-manager already exists."
else
  echo "Creating namespace cert-manager..."
  kubectl create namespace cert-manager
  echo "Namespace cert-manager created successfully."
fi

if kubectl get pods -n cert-manager --no-headers | grep -q 'Running' > /dev/null 2>&1; then
  echo "Cert-manager install already done."
else
  echo "Installing cert-manager using Helm..."
  helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --version v1.13.0 \
  --set installCRDs=true \
  --set admissionWebhooks.certManager.enabled=false \
  --set admissionWebhooks.autoGenerateCert.enabled=true

  echo "Waiting for cert-manager pods to be ready..."
  sleep 2
  kubectl wait --namespace cert-manager \
  --for=condition=Ready pods \
  --all --timeout=300s

  echo "cert-manager installation complete!"
fi

# Add the OpenTelemetry Helm repository
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts 2>&1
helm repo update 2>&1

if kubectl get pods -n opentelemetry-operator --no-headers | grep -q 'Running' > /dev/null 2>&1; then
  echo "OpenTelemetry Operator install already done."
else
  echo "Installing the OpenTelemetry Operator using Helm..."
  helm install opentelemetry-operator open-telemetry/opentelemetry-operator \
  --namespace demo \
  --version 0.24.0

  echo "Waiting for OpenTelemetry Operator pods to be ready..."
  sleep 2
  kubectl wait --namespace demo \
  --for=condition=Ready pods \
  --selector=app.kubernetes.io/name=opentelemetry-operator \
  --timeout=300s
fi

echo "OpenTelemetry Operator installation complete!"

helm repo add coralogix-charts-virtual https://cgx.jfrog.io/artifactory/coralogix-charts-virtual 2>&1
helm repo update 2>&1
helm show values coralogix-charts-virtual/otel-integration > values-crd-override.yaml

if kubectl get pods -n demo | grep -q 'coralogix-otel-collector' > /dev/null 2>&1; then
  echo "Coralogix OpenTelemetry Collector install already done."
else
  echo "Installing the Coralogix OpenTelemetry Collector using Helm..."
  helm install otel-coralogix-integration coralogix-charts-virtual/otel-integration \
    --namespace demo \
    --render-subchart-notes -f values-crd-override.yaml \
    --set global.clusterName=k8s-demo-1 \
    --set global.domain=eu2.coralogix.com \
    --set opentelemetry-cluster-collector.ports.otlp-http.enabled=true

  sleep 2
  echo "Waiting for Coralogix OpenTelemetry Collector pods to be ready..."
  kubectl wait --namespace demo \
    --for=condition=Ready pods \
    --selector=app.kubernetes.io/instance=otel-coralogix-integration  \
    --timeout=300s
fi

helm repo add stable https://charts.helm.sh/stable 2>&1
helm repo update 2>&1

if kubectl get pods -n demo | grep -q 'rabbitmq'; then
  echo "RabbitMQ install already done."
else
  echo "Installing RabbitMQ..."
  helm install my-rabbitmq bitnami/rabbitmq --namespace demo \
    --set image.repository=bitnami/rabbitmq \
    --set image.tag=4.1.3-debian-12-r1 \
    --set auth.username=admin \
    --set auth.password=adminpassword \
    --set auth.erlangCookie=secretcookie \
    --set service.type=NodePort \
    --set opentelemetry.enabled=true \
    --set opentelemetry.traces.exporter=otlp \
    --set opentelemetry.traces.otlp.endpoint=http://coralogix-opentelemetry-collector.demo.svc.cluster.local:4318 \
    --set opentelemetry.traces.otlp.insecure=true

  echo "Waiting for RabbitMQ pods to be ready..."
  sleep 2
  kubectl wait --namespace demo \
    --for=condition=Ready pods \
    --selector=app.kubernetes.io/name=rabbitmq \
    --timeout=300s
fi

# Configure Docker to use the Minikube Docker daemon to avoid pushing images
eval $(minikube docker-env)
docker build -t consumer-service:latest -f go/consumer_service/Dockerfile go
docker build -t producer-service:latest -f go/producer_service/Dockerfile go

# deploy consumer_service
echo "Deploying consumer-service..."
kubectl apply -f consumer_service_deployment.yaml

# Wait for consumer-service to be ready
echo "Waiting for consumer-service pods to be ready..."
kubectl wait --namespace demo \
  --for=condition=Ready pods \
  --selector=app=consumer-service \
  --timeout=300s 

# reset local docker
eval $(minikube docker-env -u)

