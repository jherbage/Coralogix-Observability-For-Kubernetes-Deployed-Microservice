# Coralogix-Observability-For-Kubernetes-Deployed-Microservice

This repository allows you to spin up a PoC Open Telemetry based observability demo which integrates a Kubernetes cluster deployment of a simple RabbitMQ queue message producer/consumer service. The K8s cluster's kube state metrics are also integrated.

## Design Notes

1. OpenTelemetry Operator has been used to create the OTEL resource since most organisations using otel would be using the Operator.

## Mac Prepreqs

1. Run the following command to install `curl`, `docker`, `kubectl` and `minikube` using Homebrew:

   `brew install curl docker kubectl minikube helm`

2. Verify minikube is installed:

    `minikube version`

3. You will need a Coralogix login. Trial available [here](https://dashboard.eu2.coralogix.com/#/signup). Create a k8s integration in coralogix and get the key you create. Use it here:

    `export CORALOGIX_APIKEY=<key value>`



## Start the Demo

1. Run `bash start.sh` to spin up the demo build
2. Run `bash do-demo.sh` to fire some messages at it and make the scenario happen

### Notes

* `global.clusterName` is whatever you want your k8s cluster be show up as in Coralogix. You also need to set that up in their integration.
* `global.domain` points at the coralogix APIs so I used eu2.coralogix.com as advised by the integration
* RabbitMQ isn't secured as its non production


## Clear Down the Demo

1. Run `bash teardown.sh` to clear down your minikube and start again

## TO-DO
1. Is it possible to create the k8s integration cluster in the Coralogix API programmatically?