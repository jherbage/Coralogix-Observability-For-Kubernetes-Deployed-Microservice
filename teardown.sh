#!/
set -e 

echo "Stopping Minikube..."
minikube stop

echo "Deleting Minikube cluster..."
minikube delete

echo "Clearing Minikube data..."
# Remove any residual Minikube files
rm -rf ~/.minikube

echo "Teardown complete! Minikube has been cleared of all data."

# reset local docker
eval $(minikube docker-env -u)