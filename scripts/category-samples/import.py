#!/usr/bin/env python3
"""Import prepared samples through Photoly's authenticated, resumable upload API.
The session file contains either a JSON {"token": "..."} or a raw token; never logged.
"""
import argparse
import hashlib
import json
import pathlib
import time
import urllib.request
import uuid


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--directory', type=pathlib.Path, required=True)
    ap.add_argument('--base-url', default='http://localhost:8384')
    ap.add_argument('--session-file', type=pathlib.Path, required=True)
    args = ap.parse_args()
    raw = args.session_file.read_text().strip()
    token = json.loads(raw)['token'] if raw.startswith('{') else raw
    base = args.base_url.rstrip('/')
    def call(method, path, data=None, binary=False):
        headers = {'HRD-Token': token}
        if data is not None:
            headers['Content-Type'] = 'application/octet-stream' if binary else 'application/json'
            if not binary: data = json.dumps(data).encode()
        req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
        with urllib.request.urlopen(req, timeout=120) as r: body = json.load(r)
        if body.get('msgCode') != 200: raise RuntimeError(f'{method} {path}: {body.get("msgCode")} {body.get("t")}')
        return body.get('t')
    doc = json.loads((args.directory / 'manifest.json').read_text())
    root = call('GET', '/namespace/getRoot')['nsId']
    folders = call('GET', f'/namespace/getChildren?parentId={root}')
    target = next((x for x in folders if x['nsName'] == 'Category Samples'), None)
    if target is None:
        call('POST', '/namespace/insert', {'nsParentId': root, 'nsName': 'Category Samples'})
        target = next(x for x in call('GET', f'/namespace/getChildren?parentId={root}') if x['nsName'] == 'Category Samples')
    ns = target['nsId']
    files = [f for item in doc['items'] for f in item['files']]
    # A persistent receipt prevents re-import after upload history has been pruned.
    receipt_path = args.directory / 'import-receipt.json'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}
    signature = hashlib.sha256(json.dumps([(f['name'],f['sha256']) for f in files]).encode()).hexdigest()
    scope = f'{base}:{ns}:{signature}'
    existing = call('GET', f'/photo/getByNamespace?nsId={ns}')
    if receipt.get('scope') == scope and {p['photoId'] for p in existing} >= set(receipt.get('photoIds', [])) and len(receipt.get('photoIds', [])) == 30:
        print('Already imported: 30 samples, verified receipt and current media IDs'); return
    batch = str(uuid.uuid5(uuid.NAMESPACE_URL, 'photoly-category-samples:' + scope))
    call('POST', '/uploads', {'id': batch, 'nsId': ns, 'name': 'Category Samples v1'})
    entries = []
    for f in files:
        path = args.directory / 'media' / f['name']
        if path.name != f['name'] or path.parent != args.directory / 'media': raise ValueError('Unsafe filename')
        if hashlib.sha256(path.read_bytes()).hexdigest() != f['sha256']: raise ValueError('Sample checksum changed: ' + f['name'])
        entries.append({'id': str(uuid.uuid5(uuid.UUID(batch), f['name'] + f['sha256'])), 'path': f['name'], 'size': f['size']})
    registered = call('GET', f'/uploads/{batch}/items')['items']
    known = {i['id']: i for i in registered}
    missing = [e for e in entries if e['id'] not in known]
    if missing: call('POST', f'/uploads/{batch}/items', {'items': missing})
    for entry in entries:
        if known.get(entry['id'], {}).get('state', 'pending') != 'pending': continue
        data = (args.directory / 'media' / entry['path']).read_bytes()
        call('PUT', f'/uploads/{batch}/items/{entry["id"]}/content', data, True)
        print('Uploaded:', entry['path'], flush=True)
    call('POST', f'/uploads/{batch}/finish', {})
    deadline = time.monotonic() + 600
    while time.monotonic() < deadline:
        items = call('GET', f'/uploads/{batch}/items')['items']
        failed = [i for i in items if i['state'] in ('failed', 'skipped', 'cancelled')]
        if failed: raise RuntimeError('Import failed: ' + str([(i['path'], i['state'], i.get('error')) for i in failed]))
        if all(i['state'] == 'done' for i in items):
            photos = sorted({i['photoId'] for i in items if i.get('photoId')})
            if len(photos) != 30: raise RuntimeError(f'Expected 30 logical media, got {len(photos)}; check pairing')
            receipt_path.write_text(json.dumps({'scope': scope, 'batchId': batch, 'namespaceId': ns, 'photoIds': photos}, indent=2) + '\n')
            print(f'Imported 30 logical media into Category Samples (folder {ns})'); return
        time.sleep(2)
    raise TimeoutError('Import still processing; rerun to resume the same batch')

if __name__ == '__main__': main()
