"""Independent unmodified simple-salesforce client against the HTTPS candidate."""
import os
from simple_salesforce import Salesforce

url = os.environ["CANDIDATE_URL"]
assert url.startswith("https://"), "Python candidate gate requires trusted HTTPS"
sf = Salesforce(instance_url=url, session_id=os.environ["CANDIDATE_TOKEN"], version="65.0")
record = sf.Account.create({"Name": "Python candidate"})
assert record["success"]
try:
    assert sf.Account.get(record["id"])["Name"] == "Python candidate"
    sf.Account.update(record["id"], {"Name": "Python updated"})
    assert sf.query("SELECT Id,Name FROM Account WHERE Id = '%s'" % record["id"])["records"][0]["Name"] == "Python updated"
finally:
    assert sf.Account.delete(record["id"]) == 204
print("Python HTTPS CRUD passed")
