#!/usr/bin/bash

set -e
source .env
sh apply_envsubst.sh


echo "Step 1: Applying PVC, ConfigMap, Secret..."
kubectl  apply -f base/pv.yaml
kubectl  apply -f base/pvc.yaml
kubectl  apply -f base/configmap.yaml
kubectl  apply -f base/secret.yaml
kubectl  apply -f base/service.yaml

echo "Step 2: Applying Deployment..."
kubectl  apply -f base/deployment.yaml

echo "Step 3: Waiting for Garage pod to be ready..."
kubectl  wait --for=condition=ready pod -l app=gau-garage-service -n bao-${DEPLOY_ENV}-env --timeout=120s || true

echo "Step 4: Applying Bootstrap Job..."
kubectl  delete job gau-garage-bootstrap -n bao-${DEPLOY_ENV}-env --ignore-not-found=true
kubectl  apply -f base/bootstrap-job.yaml

echo "Step 5: Applying Ingress..."
kubectl  apply -f base/ingress.yaml

echo ""
echo "=== Garage deployment complete ==="
echo "Check bootstrap job status: kubectl logs job/gau-garage-bootstrap -n bao-${DEPLOY_ENV}-env"
