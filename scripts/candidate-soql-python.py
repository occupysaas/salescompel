"""Plan 29: paging, child subqueries and aggregates through real simple-salesforce.

Records live in the P29 fixture object and are removed with its journaled table.
"""
import json
import os
from simple_salesforce import Salesforce

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://localhost:") or url.startswith("https://127.0.0.1:")
run_id = os.environ["CANDIDATE_RUN_ID"]
item = os.environ["CANDIDATE_SOQL_OBJECT"]
client = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_TOKEN"], version="65.0")
accounts = "P29 %s Account" % run_id

all_rows = client.query_all("SELECT Id FROM %s" % item)
ids = [row["Id"] for row in all_rows["records"]]
assert len(ids) == 4500, len(ids)
assert len(set(ids)) == 4500

counted = client.query("SELECT COUNT(Id) FROM %s" % item)
assert counted["totalSize"] == 4500 and counted["records"] == []

children = client.query(
    "SELECT Name, (SELECT LastName FROM Contacts) FROM Account WHERE Name LIKE '%s%%'"
    % accounts
)
assert children["totalSize"] == 12, children["totalSize"]
by_name = {account["Name"]: account for account in children["records"]}
assert len(by_name) == 12
for i in range(12):
    account = by_name["%s %d" % (accounts, i)]
    assert account["Contacts"]["totalSize"] == 1
    assert account["Contacts"]["records"][0]["LastName"] == "P29 %s Contact %d" % (run_id, i)

output = os.environ["CANDIDATE_STEP_REPORT"]
fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as stream:
    json.dump(dict(schemaVersion=1, runId=run_id, suite="schema", complete=True, exitCode=0,
                   tests=[dict(id="schema/soql-python", outcome="passed")]), stream)
    stream.flush()
    os.fsync(stream.fileno())
print("Python SOQL paging, child and aggregate passed")
