"""Plan 28: every active field type through real, unmodified simple-salesforce.

Records live in the P28 fixture object and are removed with its journaled table.
"""
import json
import os
from decimal import Decimal
from simple_salesforce import Salesforce
from simple_salesforce.exceptions import SalesforceMalformedRequest, SalesforceResourceNotFound

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://localhost:") or url.startswith("https://127.0.0.1:")
run_id = os.environ["CANDIDATE_RUN_ID"]
item = os.environ["CANDIDATE_FIELD_TYPES_OBJECT"]
client = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_TOKEN"], version="65.0")
sobject = getattr(client, item)

def exact(value):
    return Decimal(str(value))

described = {f["name"]: f for f in sobject.describe()["fields"]}
assert described["Amount__c"]["type"] == "currency" and described["Due__c"]["type"] == "date"
created = sobject.create({
    "Amount__c": 0.1 + 0.2,
    "Quantity__c": Decimal("12.345").__float__(),
    "Score__c": 7,
    "Due__c": "2026-11-11",
    "Starts__c": "2026-03-09T09:30:00+05:00",
    "Done__c": True,
    "Stage__c": "Closed",
    "Tags__c": "B;A;B",
    "Code__c": "python",
})
assert created["success"]
record_id = created["id"]
row = sobject.get(record_id)
assert exact(row["Amount__c"]) == Decimal("0.3"), row["Amount__c"]
assert exact(row["Quantity__c"]) == Decimal("12.35"), row["Quantity__c"]
assert exact(row["Score__c"]) == Decimal("7")
assert row["Due__c"] == "2026-11-11"
assert row["Starts__c"] == "2026-03-09T04:30:00.000+0000", row["Starts__c"]
assert row["Done__c"] is True and row["Stage__c"] == "Closed" and row["Tags__c"] == "A;B"
assert row["Name"].startswith("TI-")
assert sobject.update(record_id, {"Amount__c": "1234567890123456.79", "Done__c": False}) == 204
row = sobject.get(record_id)
assert row["Done__c"] is False
assert abs(exact(row["Amount__c"]) - Decimal("1234567890123456.79")) < 1
try:
    sobject.update(record_id, {"Stage__c": "Pending"})
    raise AssertionError("restricted picklist accepted an unlisted value")
except SalesforceMalformedRequest as e:
    assert e.content[0]["errorCode"] == "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST"
result = client.query("SELECT Id,Amount__c,Due__c FROM %s WHERE Code__c = 'python' LIMIT 10" % item)
assert len(result["records"]) == 1 and result["records"][0]["Due__c"] == "2026-11-11"
assert sobject.delete(record_id) == 204
try:
    sobject.get(record_id)
    raise AssertionError("deleted record still present")
except SalesforceResourceNotFound:
    pass

output = os.environ["CANDIDATE_STEP_REPORT"]
fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as stream:
    json.dump(dict(schemaVersion=1, runId=run_id, suite="schema", complete=True, exitCode=0,
                   tests=[dict(id="schema/field-types-python", outcome="passed")]), stream)
    stream.flush()
    os.fsync(stream.fileno())
print("Python field types passed")
