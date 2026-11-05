"""OAuth session CRUD through the pinned, unmodified simple-salesforce client."""
import json
import os
from simple_salesforce import Salesforce
from simple_salesforce.exceptions import SalesforceResourceNotFound

def main():
    name = os.environ["CANDIDATE_OAUTH_NAME"]
    client = Salesforce(instance_url=os.environ["CANDIDATE_URL"],
                        session_id=os.environ["CANDIDATE_OAUTH_SESSION"], version="65.0")
    base = dict(schemaVersion=1, runId=os.environ["CANDIDATE_RUN_ID"],
                operationId="oauth-python-account", principal="admin", object="Account", names=[name])
    def event(kind, record_id=None):
        value = dict(base, kind=kind)
        if record_id:
            value["id"] = record_id
        with open(os.environ["CANDIDATE_FIXTURE_JOURNAL"], "a", encoding="utf8") as stream:
            stream.write(json.dumps(value) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
    event("intent")
    row = client.Account.create({"Name": name})
    if not row.get("success"):
        raise RuntimeError("OAuth CRUD failed")
    record_id = row["id"]
    event("known", record_id)
    try:
        if client.Account.get(record_id)["Name"] != name:
            raise RuntimeError("OAuth CRUD failed")
    finally:
        if client.Account.delete(record_id) != 204:
            raise RuntimeError("OAuth CRUD failed")
        event("deleted", record_id)
    try:
        client.Account.get(record_id)
    except SalesforceResourceNotFound:
        event("absent", record_id)
    else:
        raise RuntimeError("OAuth CRUD failed")
    print("OAuth session CRUD passed")

if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Never emit client exceptions containing request bodies or tokens.
        raise SystemExit("OAuth session CRUD failed")
