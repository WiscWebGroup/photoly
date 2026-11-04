#!/usr/bin/env python3
"""Validate the committed contract and optional real Go response samples.

Install scripts/requirements-openapi.txt. No network access or generation occurs.
"""
import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[1]
METHODS = {'get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--samples', action='append', default=[], help='JSON response samples from Go integration tests')
    args = parser.parse_args()
    spec = json.loads((ROOT / 'photoly_server/openapi.json').read_text())
    assert spec['openapi'] == '3.2.0', 'Keep the published OpenAPI version'
    failures = []
    operations = set()
    examples = 0
    def validate(schema, value, label):
        nonlocal examples
        examples += 1
        validator = Draft202012Validator({'components': spec['components'], **schema})
        for error in validator.iter_errors(value):
            # Keep nested anyOf failures actionable without dumping large bodies.
            detail = '; '.join(e.json_path + ': ' + e.message[:100] for e in error.context[:4])
            failures.append(f'{label}: {error.json_path}: {error.message[:300]}' + (f' ({detail})' if detail else ''))
    def walk(node, where):
        if isinstance(node, list):
            for i, item in enumerate(node): walk(item, f'{where}/{i}')
        elif isinstance(node, dict):
            if '$ref' in node:
                ref = node['$ref']
                if not ref.startswith('#/'):
                    failures.append(f'{where}: external reference is not bundled: {ref}')
                else:
                    target = spec
                    try:
                        for key in ref[2:].split('/'): target = target[key.replace('~1', '/').replace('~0', '~')]
                    except (KeyError, TypeError): failures.append(f'{where}: unresolved {ref}')
            if 'schema' in node:
                Draft202012Validator.check_schema(node['schema'])
                if 'example' in node: validate(node['schema'], node['example'], where + '/example')
                for name, ex in node.get('examples', {}).items():
                    if 'value' in ex: validate(node['schema'], ex['value'], where + '/examples/' + name)
            for key, value in node.items():
                if key not in ('example', 'examples'): walk(value, where + '/' + key)
    for path, item in spec['paths'].items():
        for method, op in item.items():
            if method not in METHODS: continue
            label = method.upper() + ' ' + path
            oid = op.get('operationId')
            if not oid or oid in operations: failures.append(f'{label}: missing/duplicate operationId {oid}')
            operations.add(oid)
            params = item.get('parameters', []) + op.get('parameters', [])
            names = [p['name'] for p in params if p.get('in') == 'path']
            if set(names) != set(re.findall(r'\{([^}]+)\}', path)): failures.append(f'{label}: path parameters do not match template')
            keys = [(p['in'],p['name']) for p in params]
            if len(keys) != len(set(keys)): failures.append(f'{label}: duplicate parameters')
            if any(not p.get('required') for p in params if p.get('in') == 'path'): failures.append(f'{label}: optional path parameter')
            for security in op.get('security', spec.get('security', [])):
                for name in security:
                    if name not in spec['components']['securitySchemes']: failures.append(f'{label}: unknown security scheme {name}')
    for schema in spec['components']['schemas'].values():
        Draft202012Validator.check_schema(schema)
    walk(spec, '#')
    # Run the credential/resource generator in a disposable tree. A later
    # regeneration must not undo reviewed response or maintenance contracts.
    with tempfile.TemporaryDirectory(prefix='photoly-contract-') as temp:
        root = Path(temp)
        for relative in ('scripts/update-credential-openapi.py', 'photoly_server/openapi.json', 'photoly_frontend_vue/src/lib/credentialEndpoints.json'):
            (root / relative).parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / relative, root / relative)
        subprocess.run([sys.executable, str(root / 'scripts/update-credential-openapi.py')], check=True)
        if json.loads((root / 'photoly_server/openapi.json').read_text()) != spec:
            failures.append('Credential/resource contracts differ from their generator; regenerate and review the diff.')
    for filename in args.samples:
        for sample in json.loads(Path(filename).read_text()):
            label = sample['method'].upper() + ' ' + sample['path'] + ' HTTP ' + str(sample['status'])
            try:
                response = spec['paths'][sample['path']][sample['method']]['responses'][str(sample['status'])]
                schema = response['content']['application/json']['schema']
            except KeyError:
                failures.append(label + ': undocumented JSON response')
                continue
            validate(schema, sample['body'], label)
    if failures:
        raise SystemExit('\n'.join(failures))
    print(f'OpenAPI: {len(operations)} operations; references, parameters and {examples} examples/real responses valid.')

if __name__ == '__main__': main()
