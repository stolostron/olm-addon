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
echo "running e2e tests"
set -o pipefail
ssh "${OPT[@]}" "$HOST" "export GOROOT=/usr/lib/golang; export PATH=\$GOROOT/bin:\$PATH; echo \$PATH && cd /tmp/olm-addon && go version && kind version && go mod download && make e2e" 2>&1 | tee $ARTIFACT_DIR/test.log
if [[ $? -ne 0 ]]; then
  echo "Failure"
  cat $ARTIFACT_DIR/test.log
  echo "======================= controller logs ======================="
  ssh "${OPT[@]}" "$HOST" "cd /tmp/olm-addon && rundir=\$(cat run-dir.txt); cat \$rundir/addon-manager.log"
  exit 1
fi
