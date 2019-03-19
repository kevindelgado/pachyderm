#!/bin/bash
set -exo pipefail

# TODO: parameterize/prompt these
#CLUSTER_NAME="script-cluster-7"
#GCP_ZONE="us-west1-a"
#MACHINE_TYPE="n1-highmem-8"
#STORAGE_SIZE=10
#BUCKET_NAME=script-bucket-7
#REPORT_OUTPUT=script-report-7

### Cloud k8s

# Create GKE cluster
gcloud config set compute/zone ${GCP_ZONE}
gcloud config set container/cluster ${CLUSTER_NAME}
gcloud container clusters create ${CLUSTER_NAME} --scopes storage-rw --machine-type ${MACHINE_TYPE}

# Verify success of cluster creation

# TODO: run list command and check that name=CLUSTER_NAME and status is RUNNING.
gcloud container clusters list
#gcloud container clusters describe ${CLUSTER_NAME}

# Update cluster role bindings
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value account)

# Setup storage resources
gsutil mb gs://${BUCKET_NAME}

# TODO: Verify success of storage resources
gsutil ls

### Pachyderm

# Install pachctl (TODO: clean this up, maybe use go install)
wget https://github.com/pachyderm/pachyderm/releases/download/v1.8.6/pachctl_1.8.6_darwin_amd64.zip -O dwnld && unzip -p dwnld > pachctl_temp && mv pachctl_temp ~/go/bin/pachctl && chmod +x ~/go/bin/pachctl && rm dwnld

# TODO: Verify pachctl install
pachctl version --client-only

# Deploy pachyderm
pachctl deploy google ${BUCKET_NAME} ${STORAGE_SIZE} --dynamic-etcd-nodes=1

# TODO: Verify success of pachyderm deploy (check that etcd and pachd are ready)
kubectl get pods
echo "waiting for pach cluster to spin up..."
set +ex
while :
do
    ETCD_CR=$(kubectl get pods -l app=etcd -o=jsonpath='{.items..status.conditions[2].type}')
    ETCD_STATUS=$(kubectl get pods -l app=etcd -o=jsonpath='{.items..status.conditions[2].status}')
    PACHD_CR=$(kubectl get pods -l app=pachd -o=jsonpath='{.items..status.conditions[2].type}')
    PACHD_STATUS=$(kubectl get pods -l app=pachd -o=jsonpath='{.items..status.conditions[2].status}')


    if [ "$ETCD_CR" == "ContainersReady" ] && [ "$ETCD_STATUS" == "True" ] && [ "$PACHD_CR" == "ContainersReady" ] && [ "$PACHD_STATUS" == "True" ]; then
        echo "pach cluster is ready"
        break
    fi
done
set -ex


# Port forward
set +e
killall pachctl 
set -e
pachctl port-forward &