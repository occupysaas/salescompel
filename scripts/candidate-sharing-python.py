"""Plan 34: under a Private Account default, a manager sees a subordinate's record through real simple-salesforce; a peer outside the hierarchy does not."""
import os
from simple_salesforce import Salesforce
from simple_salesforce.exceptions import SalesforceResourceNotFound

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://localhost:") or url.startswith("https://127.0.0.1:")
account = os.environ["CANDIDATE_SHARING_ACCOUNT"]
soql = "SELECT Id FROM Account WHERE Id = '%s'" % account


def client(token):
    return Salesforce(instance_url=url, session_id=os.environ[token], version="65.0")


manager = client("CANDIDATE_SHARING_MANAGER_TOKEN")
assert manager.query(soql)["totalSize"] == 1
assert manager.Account.get(account)["Id"] == account
assert manager.Account.update(account, {"Description": "p34 manager"}) == 204

peer = client("CANDIDATE_SHARING_PEER_TOKEN")
assert peer.query(soql)["totalSize"] == 0
try:
    peer.Account.get(account)
except SalesforceResourceNotFound as error:
    assert error.content[0]["errorCode"] == "NOT_FOUND", error.content
else:
    raise AssertionError("peer read a private record")
print("Python role hierarchy passed")
