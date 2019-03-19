#!/bin/bash
set -exo pipefail

# TODO: parameterize/prompt these
export CLUSTER_NAME="script-cluster-8"
export GCP_ZONE="us-west1-a"
export MACHINE_TYPE="n1-highmem-8"
export STORAGE_SIZE=10
export BUCKET_NAME=script-bucket-8
REPORT_OUTPUT=script-report-8
LOADTEST_ROOT=split

# Spin up a running pachyderm cluster on GCP (TODO: or aws/msft, configurable number of clusters)
~/go/src/github.com/pachyderm/pachyderm/src/testing/loadtest/gcp.sh 

### Loadtest

# make supervisor docker image
cd ${GOPATH}/src/github.com/pachyderm/pachyderm/src/testing/loadtest/${LOADTEST_ROOT} && make docker-build

# TODO: Allow for custom config of supervisor yaml

# apply supervisor yaml 
kubectl apply -f kube/supervisor.yaml

# generate report (pipe supervisor logs to file)

# TODO: if supervisor is in completed state:
Echo "Waiting for supervisor to finish..."
set +x
while :
do
    STATUS=$(kubectl get pods -l app=split-loadtest-supervisor -o=jsonpath='{.items..status.containerStatuses..state.terminated.reason}')
    if [ "${STATUS}" == "Completed" ]; then
        echo "supervisor completed"
        break
    fi
done
set -x
# announce where the report can be found
kubectl logs split-loadtest-supervisor > ${REPORT_OUTPUT}
echo "Report at ${REPORT_OUTPUT}"

# Teardown cluster with prompt (TODO: or flag for no-prompt/no-teardown)
echo -n "Delete the cluster and storage resources? (y/N)"
read answer
if [ "$answer" != "${answer#[Yy]}" ]; then
    echo "Deleting the cluster"
    gcloud container clusters delete ${CLUSTER_NAME}
    gsutil rb gs://${BUCKET_NAME}
else
    echo "Cluster not deleted"
fi
