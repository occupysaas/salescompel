#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
: "${DATABASE_URL:?Set DATABASE_URL to a disposable PostgreSQL database}"
: "${CANDIDATE_DISPOSABLE:?Set CANDIDATE_DISPOSABLE=yes to permit synthetic fixture writes}"
[[ "$CANDIDATE_DISPOSABLE" == yes ]] || exit 2
scratch=$(mktemp -d)
server_pid=''
cleanup() {
  if [[ -n "$server_pid" ]]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  # Preserve logs on failure, remove only this script's temporary directory on success.
  if [[ "$1" == 0 ]]; then rm -rf "$scratch"; else printf 'Candidate logs: %s\n' "$scratch" >&2; fi
}
trap 'cleanup $?' EXIT
export CANDIDATE_URL="https://localhost:${CANDIDATE_PORT:-58443}"
run_id="$(openssl rand -hex 6)"
export SALESCOMPEL_TENANT="00D${run_id}EAA"
export CANDIDATE_TOKEN="$SALESCOMPEL_TENANT!$(openssl rand -hex 24)"
export CANDIDATE_RESTRICTED_TOKEN="$(openssl rand -hex 24)"
export CANDIDATE_OTHER_TOKEN="$(openssl rand -hex 24)"
export SALESCOMPEL_USER="005000000000001AAA"
export SALESCOMPEL_ROLE=admin
export NODE_EXTRA_CA_CERTS="$scratch/cert.pem" REQUESTS_CA_BUNDLE="$scratch/cert.pem"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$scratch/key.pem" -out "$scratch/cert.pem" -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' > "$scratch/tls.log" 2>&1
go build -o "$scratch/salescompel" ./cmd/salescompel
SALESCOMPEL_TOKEN="$CANDIDATE_TOKEN" "$scratch/salescompel" provision
SALESCOMPEL_TOKEN="$CANDIDATE_RESTRICTED_TOKEN" SALESCOMPEL_ROLE=restricted SALESCOMPEL_USER=005000000000002AAA "$scratch/salescompel" provision
SALESCOMPEL_TOKEN="$CANDIDATE_OTHER_TOKEN" SALESCOMPEL_TENANT="other-$run_id" "$scratch/salescompel" provision
start_server() {
 "$scratch/salescompel" serve -addr "127.0.0.1:${CANDIDATE_PORT:-58443}" -tls-cert "$scratch/cert.pem" -tls-key "$scratch/key.pem" > "$scratch/server.log" 2>&1 &
 server_pid=$!
 for attempt in {1..100}; do
   if curl --silent --fail --cacert "$scratch/cert.pem" "$CANDIDATE_URL/healthz" > /dev/null; then return; fi
   kill -0 "$server_pid" 2>/dev/null || { cat "$scratch/server.log"; return 1; }
   sleep 0.1
 done
 return 1
}
start_server
RUN_CANDIDATE=1 npm --prefix compat/clients/node exec -- tsx --test compat/clients/node/test/candidate.test.ts
"${CANDIDATE_PYTHON:-compat/clients/python/.venv/bin/python}" scripts/candidate-python.py
# Verify persistence across an actual service process restart.
curl --silent --show-error --fail --cacert "$scratch/cert.pem" -H "Authorization: Bearer $CANDIDATE_TOKEN" -H 'Content-Type: application/json' -d '{"Name":"Restart probe"}' "$CANDIDATE_URL/services/data/v65.0/sobjects/Account" > "$scratch/record.json"
record_id=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["id"])' "$scratch/record.json")
kill "$server_pid"; wait "$server_pid"; server_pid=''
start_server
curl --silent --show-error --fail --cacert "$scratch/cert.pem" -H "Authorization: Bearer $CANDIDATE_TOKEN" "$CANDIDATE_URL/services/data/v65.0/sobjects/Account/$record_id" > "$scratch/read.json"
python3 -c 'import json,sys;assert json.load(open(sys.argv[1]))["Name"]=="Restart probe"' "$scratch/read.json"
curl --silent --show-error --fail --cacert "$scratch/cert.pem" -X DELETE -H "Authorization: Bearer $CANDIDATE_TOKEN" "$CANDIDATE_URL/services/data/v65.0/sobjects/Account/$record_id"
# Verify native describe again after an actual process restart.
RUN_CANDIDATE=1 npm --prefix compat/clients/node exec -- tsx --test compat/clients/node/test/candidate.test.ts
# Isolate Salesforce CLI credential files to the disposable test directory.
export SF_STATE_FOLDER="$scratch/sf" SF_ACCESS_TOKEN="$CANDIDATE_TOKEN"
compat/clients/node/node_modules/.bin/sf org login access-token --instance-url "$CANDIDATE_URL" --alias m2-candidate --no-prompt --json > "$scratch/cli-login.json"
compat/clients/node/node_modules/.bin/sf data query --target-org m2-candidate --query 'SELECT Id,Name FROM Account LIMIT 1' --api-version 65.0 --json > "$scratch/cli-query.json"
python3 -c 'import json,sys;r=json.load(open(sys.argv[1]));assert r["status"]==0;assert r["result"]["done"] is True' "$scratch/cli-query.json"
npm --prefix web run test:e2e
printf 'Candidate connector, restart, CLI and browser gates passed. No Salesforce reference certification.\n'
