IMAGE     = libra-api
TAG       = latest
NAMESPACE = libra
K8S       = k8s

.PHONY: build deploy delete restart logs status all

## Build the Docker image
build:
	docker build -t $(IMAGE):$(TAG) .

## Apply all manifests in dependency order
deploy:
	kubectl apply -f $(K8S)/namespace.yaml
	kubectl apply -f $(K8S)/secret.yaml
	kubectl apply -f $(K8S)/postgres-configmap.yaml
	kubectl apply -f $(K8S)/postgres-pvc.yaml
	kubectl apply -f $(K8S)/postgres-deployment.yaml
	kubectl apply -f $(K8S)/postgres-service.yaml
	kubectl apply -f $(K8S)/app-deployment.yaml
	kubectl apply -f $(K8S)/app-service.yaml

## Build image then deploy everything
all: build deploy

## Tear down — deletes the entire namespace and all resources inside it
delete:
	kubectl delete namespace $(NAMESPACE)

## Force a rolling restart of both deployments (picks up a rebuilt image)
restart:
	kubectl rollout restart deployment/postgres   -n $(NAMESPACE)
	kubectl rollout restart deployment/libra-api  -n $(NAMESPACE)

## Wait for all pods to be ready
wait:
	kubectl rollout status deployment/postgres  -n $(NAMESPACE)
	kubectl rollout status deployment/libra-api -n $(NAMESPACE)

## Tail logs from the API
logs:
	kubectl logs -f deployment/libra-api -n $(NAMESPACE)

## Show pods and services
status:
	kubectl get pods,svc -n $(NAMESPACE)
