"""Plan 33: a Read Only user through real simple-salesforce can query but not create."""
import os
from simple_salesforce import Salesforce
from simple_salesforce.exceptions import SalesforceError

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://localhost:") or url.startswith("https://127.0.0.1:")
client = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_PERMISSIONS_TOKEN"], version="65.0")

rows = client.query("SELECT Id, Name FROM Account LIMIT 1")
assert rows["done"] is True

try:
    client.Account.create({"Name": os.environ["CANDIDATE_PERMISSIONS_NAME"]})
except SalesforceError as error:
    assert error.status == 403, error.status
    assert error.content[0]["errorCode"] == "INSUFFICIENT_ACCESS_OR_READONLY", error.content
else:
    raise AssertionError("Read Only create succeeded")
print("Python Read Only refusal passed")
