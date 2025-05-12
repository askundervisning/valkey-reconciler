Pulled: registry-1.docker.io/bitnamicharts/valkey:3.0.4
Digest: sha256:102cb2961f05cd7086bc11fb6b44a2d7084ea0f0538c68cc0e186bfee68633df
NAME: vk
LAST DEPLOYED: Fri May  9 16:16:17 2025
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
CHART NAME: valkey
CHART VERSION: 3.0.4
APP VERSION: 8.1.1

Did you know there are enterprise versions of the Bitnami catalog? For enhanced secure software supply chain features, unlimited pulls from Docker, LTS support, or application customization, see Bitnami Premium or Tanzu Application Catalog. See https://www.arrow.com/globalecs/na/vendors/bitnami for more information.

** Please be patient while the chart is being deployed **

Valkey can be accessed on the following DNS names from within your cluster:

    vk-valkey-primary.default.svc.cluster.local for read/write operations (port 6379)
    vk-valkey-replicas.default.svc.cluster.local for read-only operations (port 6379)



To get your password run:

    export VALKEY_PASSWORD=$(kubectl get secret --namespace default vk-valkey -o jsonpath="{.data.valkey-password}" | base64 -d)

To connect to your Valkey server:

1. Run a Valkey pod that you can use as a client:

   kubectl run --namespace default valkey-client --restart='Never'  --env VALKEY_PASSWORD=$VALKEY_PASSWORD  --image docker.io/bitnami/valkey:8.1.1-debian-12-r0 --command -- sleep infinity

   Use the following command to attach to the pod:

   kubectl exec --tty -i valkey-client \
   --namespace default -- bash

2. Connect using the Valkey CLI:
   REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli -h vk-valkey-primary
   REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli -h vk-valkey-replicas

To connect to your database from outside the cluster execute the following commands:

    kubectl port-forward --namespace default svc/vk-valkey-primary 6379:6379 &
    REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli -h 127.0.0.1 -p 6379

WARNING: There are "resources" sections in the chart not set. Using "resourcesPreset" is not recommended for production. For production installations, please set the following values according to your workload needs:
  - replica.resources
  - primary.resources
+info https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

