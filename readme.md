# mongodb-backup-launcher

An opinionated wrapper app designed to provision MongoDB backup jobs into a Karpenter backed AWS EKS cluster which is self-hosting a MongoDB cluster.
In particular, it is designed to solve two key issues:

1. Cross AWS AZ data transfer costs can be high when working with larger data sets. Mitigates this by scheduling a backup pod into the same AWS availability zone as a secondary MongoDB replica and targeting it directly
2. If you need to take frequent backups (e.g. hourly) this can cause lots of Karpenter initiated pod evictions as nodes scale out and in, which could impact availability. Mitigates this by scheduling onto a dedicated Karpenter NodePool with a backup taint

This app is designed to be run as K8s CronJob which it then creates K8s jobs in the required AZ after querying the MongoDB cluster.

## Running locally

### Pre-reqs

```bash
# Set envars
export LOG_LEVEL=debug                                                      # optional - defaults to info level
export EXCLUDE_REPLICA=mongodb-2.mongodb.database.svc.cluster.local:27017   # optional - if you want to exclude a particular secondary replica for any reason
export MONGODB_URI=mongodb://localhost:27017/?directConnection=true         # MongoDB endpoint. Use localhost and directConnection if going via kubectl port-forward connection
export MONGODB_USERNAME=<username>                                          # Username for connecting to the DB
export MONGODB_PASSWORD=<password>                                          # Password for connecting to the DB
export DOCKER_IMAGE_URI=<repo>:<tag>                                        # Docker image that is run in the provisioned K8s job. Should perform the actual backup e.g. mongodump
export RUNNING_LOCALLY=true                                                 # Use a local kubeconfig rather than in-cluster config for the K8s client

# Port forward to any of the MongoDB pods in the replica set
kubectl -n database port-forward sts/mongodb 27017:27017 &

# Run app locally
go run ./cmd/main.go
```