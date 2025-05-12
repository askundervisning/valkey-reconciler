IMAGE_NAME=valkey-reconciler
CLUSTER_NAME=ask-cluster

build:
	docker build -t $(IMAGE_NAME):latest .

deploy:
	k3d image import -c $(CLUSTER_NAME)  $(IMAGE_NAME):latest 
	kubectl delete pods -l app=$(IMAGE_NAME)
	kubectl apply -f $(IMAGE_NAME)-deployment.yaml
	kubectl get pods -l app=$(IMAGE_NAME)


