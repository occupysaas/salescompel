"""Independent live connector probe. All credentials arrive through the environment."""
import json
import os
import sys
from importlib.metadata import version
from pathlib import Path


def main():
    if sys.argv[1:] == ['--version']:
        print(version('simple-salesforce'))
        return 0
    from simple_salesforce import Salesforce
    from requests import Session

    class BoundedSession(Session):
        def request(self, *args, **kwargs):
            kwargs.setdefault('timeout', 30)
            return super().request(*args, **kwargs)

    result = {'version': version('simple-salesforce'), 'assertions': [], 'observations': {}, 'identities': {}, 'cleanup': []}
    record_id = None
    sf = Salesforce(instance_url=os.environ['COMPAT_PROBE_URL'], session_id=os.environ['COMPAT_PROBE_TOKEN'], version=os.environ['COMPAT_API_VERSION'], session=BoundedSession())
    try:
        name = 'SC ' + os.environ['COMPAT_RUN_ID'] + ' python'
        with open(os.environ['COMPAT_PROBE_JOURNAL'], 'a', encoding='utf-8') as journal:
            journal.write(json.dumps({'intent': 'create', 'type': 'Account', 'fields': {'Name': name}}) + '\n')
        row = sf.Account.create({'Name': name})
        record_id = row['id']
        with open(os.environ['COMPAT_PROBE_JOURNAL'], 'a', encoding='utf-8') as journal:
            journal.write(json.dumps({'resource': 'Account:' + record_id}) + '\n')
        result['identities']['account'] = record_id
        before = sf.Account.get(record_id)
        result['assertions'].append({'name': 'Python create/read', 'passed': before['Name'] == name})
        sf.Account.update(record_id, {'Name': 'Python updated'})
        rows = sf.query("SELECT Id,Name FROM Account WHERE Id = '%s'" % record_id)
        result['observations'] = {'records': [{'Id': r['Id'], 'Name': r['Name']} for r in rows['records']]}
        result['assertions'].append({'name': 'Python update/query', 'passed': len(rows['records']) == 1 and rows['records'][0]['Name'] == 'Python updated'})
    except Exception as exc:
        # Deliberately omit raw HTTP/auth exception text. Parent redacts results too.
        result['error'] = type(exc).__name__
    finally:
        if record_id:
            try:
                status = sf.Account.delete(record_id)
                result['cleanup'].append({'resource': 'Account:' + record_id, 'outcome': 'passed' if status == 204 else 'failed'})
            except Exception as exc:
                result['cleanup'].append({'resource': 'Account:' + record_id, 'outcome': 'failed', 'reason': type(exc).__name__})
    result['outcome'] = 'passed' if result['assertions'] and all(a['passed'] for a in result['assertions']) and not result.get('error') and all(c['outcome'] == 'passed' for c in result['cleanup']) else 'failed'
    print(json.dumps(result))
    return 0


if __name__ == '__main__':
    sys.exit(main())
