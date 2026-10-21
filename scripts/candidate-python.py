"""Real unmodified simple-salesforce client; private fixture/step evidence."""
import json
import os
import uuid
from simple_salesforce import Salesforce
from simple_salesforce.exceptions import SalesforceResourceNotFound, SalesforceError

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://localhost:") or url.startswith("https://127.0.0.1:")
run_id = os.environ.get("CANDIDATE_RUN_ID", uuid.uuid4().hex[:24])
name = "p15-" + run_id + "-python"
client = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_TOKEN"], version="65.0")
other = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_OTHER_TOKEN"], version="65.0")
owned = []
steps = []
def event(value):
    path = os.environ.get("CANDIDATE_FIXTURE_JOURNAL")
    if path:
        with open(path, "a", encoding="utf8") as stream:
            stream.write(json.dumps(value) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
def create(object_name, data):
    e = dict(schemaVersion=1, runId=run_id, operationId="python-" + object_name,
             principal="admin", object=object_name, names=[name, name + " updated"], kind="intent")
    event(e)
    row = getattr(client, object_name).create(data)
    assert row["success"]
    owned.append((object_name, row["id"]))
    event(dict(e, kind="known", id=row["id"]))
    return row["id"]
try:
    account = create("Account", {"Name": name})
    contact = create("Contact", {"LastName": name, "AccountId": account})
    result = client.query("SELECT Id,LastName,Account.Name FROM Contact WHERE AccountId = '%s'" % account)
    assert len(result["records"]) == 1 and result["records"][0]["Id"] == contact
    assert result["records"][0]["Account"]["Name"] == name
    steps.append("python/relationship")
    client.Account.update(account, {"Name": name + " updated"})
    assert client.Account.get(account)["Name"] == name + " updated"
    steps.append("python/update")
    try:
        other.Account.update(account, {"Name": "foreign"})
        raise AssertionError("cross-tenant update accepted")
    except SalesforceResourceNotFound:
        pass
    assert client.Account.get(account)["Name"] == name + " updated"
    steps.append("python/denied")
finally:
    for object_name, record_id in sorted(owned, key=lambda row: row[0] != "Contact"):
        assert getattr(client, object_name).delete(record_id) == 204
        try:
            getattr(client, object_name).get(record_id)
            raise AssertionError("deleted fixture still present")
        except SalesforceResourceNotFound:
            pass
steps.append("python/cleanup")
output = os.environ.get("CANDIDATE_STEP_REPORT")
if output:
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as stream:
        json.dump(dict(schemaVersion=1, runId=run_id, suite="python", complete=True, exitCode=0,
                       tests=[dict(id=step, outcome="passed") for step in steps]), stream)
        stream.flush()
        os.fsync(stream.fileno())
print("Python candidate relationship, policy and cleanup passed")
