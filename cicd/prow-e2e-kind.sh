#!/bin/bash

KEY="$SHARED_DIR/private.pem"
chmod 400 "$KEY"

IP="$(cat "$SHARED_DIR/public_ip")"
HOST="ec2-user@$IP"
OPT=(-q -o "UserKnownHostsFile=/dev/null" -o "StrictHostKeyChecking=no" -i "$KEY")

scp "${OPT[@]}" -r ../olm-addon "$HOST:/tmp/olm-addon"
ssh "${OPT[@]}" "$HOST" sudo sed -i 's~::1~#::1~g' /etc/hosts
ssh "${OPT[@]}" "$HOST" sudo yum install git golang -y
# to run as normal user
# ssh "${OPT[@]}" "$HOST" sudo usermod -a -G docker '$USER'

system=$(ssh "${OPT[@]}" "$HOST" "uname")
echo "operating system: $system"

# Install the latest kubectl version
if [[ "$system" == "Linux" ]]; then
    ssh "${OPT[@]}" "$HOST" "curl -LO \"https://dl.k8s.io/release/\$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl\""
elif [[ "$system" == "Darwin" ]]; then
    ssh "${OPT[@]}" "$HOST" "curl -LO \"https://dl.k8s.io/release/\$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl\""
fi
ssh "${OPT[@]}" "$HOST" "chmod +x ./kubectl; sudo mv ./kubectl /usr/bin/kubectl; kubectl version"

# Install the kind v0.20.0
if [[ "$system" == "Linux" ]]; then
    ssh "${OPT[@]}" "$HOST" "curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64"
elif [[ "$system" == "Darwin" ]]; then
    ssh "${OPT[@]}" "$HOST" "curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-darwin-arm64"
fi
ssh "${OPT[@]}" "$HOST" "chmod +x ./kind; sudo mv ./kind /usr/bin/kind; kind version"

# Increase resources for the kind cluster running OCM
ssh "${OPT[@]}" "$HOST" "sudo sh -c 'echo \"fs.inotify.max_user_watches=2097152\" >> /etc/sysctl.conf && echo \"fs.inotify.max_user_instances=1024\" >> /etc/sysctl.conf && sysctl -p /etc/sysctl.conf'"

echo "running e2e tests"
set -o pipefail
ssh "${OPT[@]}" "$HOST" "export GOROOT=/usr/lib/golang; export PATH=\$GOROOT/bin:/usr/local/bin:\$PATH; echo \$PATH && cd /tmp/olm-addon && go version && kind version && go mod download && make build && export DEBUG=true; make e2e" 2>&1 | tee $ARTIFACT_DIR/test.log
if [[ $? -ne 0 ]]; then
  echo "Failure"
  cat $ARTIFACT_DIR/test.log
  ssh "${OPT[@]}" "$HOST" "rundir=\$(cat /tmp/olm-addon/run-dir.txt); kubectl get pods --kubeconfig=\$rundir/olm-addon-e2e.kubeconfig -A; kubectl get ManagedClusterAddOn --kubeconfig=\$rundir/olm-addon-e2e.kubeconfig -A -o yaml"
  echo "======================= controller logs ======================="
  ssh "${OPT[@]}" "$HOST" "cd /tmp/olm-addon && rundir=\$(cat run-dir.txt); tail -800 \$rundir/addon-manager.log"
  
  exit 1
fi
