IMAGE_NAME=reconciler-test-server
CLUSTER_NAME=ask-cluster

build:
	docker build -t $(IMAGE_NAME):latest .

deploy:
	k3d image import -c $(CLUSTER_NAME)  $(IMAGE_NAME):latest 
	kubectl delete pods -l app=test-server
	kubectl apply -f test-server-deployment.yaml -f test-server-ingress.yaml
	kubectl get pods -l app=test-server
	kubectl get svc -l app=test-server
	kubectl get ingress -l app=test-server


