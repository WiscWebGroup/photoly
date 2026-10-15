"""Documentation coverage, schema accuracy, examples and offline UI assets."""
import hashlib
import json
import re
from pathlib import Path
from unittest.mock import patch
import pytest
from openapi_spec_validator import validate
from app import create_app
from app.docs import build_spec
from tests.openapi_assertions import validate_value

CONTRACTS = Path(__file__).parent / 'contracts'
DOCS = Path(__file__).parents[1] / 'app/docs'


def test_openapi_32_is_valid_and_covers_exact_legacy_routes():
    spec = build_spec()
    validate(spec)
    assert spec['openapi'] == '3.2.0'
    expected = {(row['method'].lower(), row['path']) for row in json.loads((CONTRACTS / 'spring_routes.json').read_text())['routes']}
    actual = {(method, path) for path, methods in spec['paths'].items() for method in methods}
    assert len(expected) == 90
    assert actual == expected | {('get', '/healthz'), ('post', '/photo/replace')}
    ids = [op['operationId'] for methods in spec['paths'].values() for op in methods.values()]
    assert len(ids) == len(set(ids)) == 92
    for path, methods in spec['paths'].items():
        for op in methods.values():
            assert op['summary'] and op['description'] and op['tags']
            assert '200' in op['responses']
            placeholders = set(re.findall(r'{(\w+)}', path))
            assert placeholders == {p['name'] for p in op.get('parameters', []) if p['in'] == 'path'}


def test_every_documented_example_matches_its_schema():
    spec = build_spec()
    for path, methods in spec['paths'].items():
        for op in methods.values():
            contents = [op.get('requestBody', {}).get('content', {})]
            contents += [response.get('content', {}) for response in op['responses'].values()]
            for content in contents:
                for media in content.values():
                    examples = [v['value'] for v in media.get('examples', {}).values()]
                    if 'example' in media:
                        examples.append(media['example'])
                    for value in examples:
                        try:
                            validate_value(value, media['schema'])
                        except Exception as error:
                            raise AssertionError(f'{path}: invalid documented example {value}') from error
            for parameter in op.get('parameters', []):
                if 'example' in parameter:
                    validate_value(parameter['example'], parameter['schema'])


def test_entity_fields_match_independent_spring_snapshot():
    spec = build_spec()
    expected = json.loads((CONTRACTS / 'spring_entities.json').read_text())
    types = json.loads((CONTRACTS / 'spring_field_types.json').read_text())
    for name, fields in expected.items():
        schema = spec['components']['schemas'][name]
        assert set(schema['properties']) == set(fields)
        assert set(schema['required']) == set(fields)
        for field, java_type in types[name].items():
            definition = schema['properties'][field]
            if field == 'password':
                assert definition['type'] == 'null'
            else:
                scalar = definition.get('anyOf', [definition])[0]
                assert scalar['type'] == {'Integer': 'integer', 'Double': 'number', 'String': 'string', 'LocalDateTime': 'string'}[java_type]
                if java_type == 'LocalDateTime':
                    validate_value('2026-01-15T12:30:00', definition)
                    with pytest.raises(Exception):
                        validate_value('2026-01-15T12:30:00Z', definition)


def test_auth_uploads_lists_ranges_and_special_contracts():
    spec = build_spec()
    assert spec['servers'] == [{'url': '/', 'description': 'Current backend origin'}]
    scheme = spec['components']['securitySchemes']['SessionToken']
    assert (scheme['type'], scheme['in'], scheme['name']) == ('apiKey', 'header', 'HRD-Token')
    for prefix in ('photo', 'cred'):
        for suffix, expected in ((('insert' if prefix == 'photo' else 'uploadPhoto/{token}'), {'file', 'photo'}),
                                 (('inserts' if prefix == 'photo' else 'uploadPhotos/{token}'), {'files', 'photosStr'})):
            op = spec['paths'][f'/{prefix}/{suffix}']['post']
            schema = op['requestBody']['content']['multipart/form-data']['schema']
            assert set(schema['properties']) == set(schema['required']) == expected
            metadata = schema['properties']['photo' if 'photo' in expected else 'photosStr']
            assert metadata['type'] == 'string'
            decoded = json.loads(metadata['example'])
            for item in decoded if isinstance(decoded, list) else [decoded]:
                validate_value(item, {'$ref': '#/components/schemas/PhotoUploadMetadata'})
    tags = spec['paths']['/photo/getByTags']['get']['parameters'][0]
    assert (tags['style'], tags['explode']) == ('form', True)
    assert 'comma-separated' in tags['description']
    for path in ('/photo/renderV/{token}', '/photo/renderVToken'):
        op = spec['paths'][path]['get']
        assert {'200', '206', '416'} <= set(op['responses'])
        assert any(p['name'] == 'Range' and p['in'] == 'header' for p in op['parameters'])
    for path in ('/namespace/downloadNs/{token}', '/gallery/downloadGa/{token}', '/photo/download/{token}'):
        assert 'Content-Disposition' in spec['paths'][path]['get']['responses']['200']['headers']
    for path in ('/admin/stop', '/user/deleteUser'):
        op = next(iter(spec['paths'][path].values()))
        assert 'DESTRUCTIVE' in op['description']
    assert spec['paths']['/cred/uploadPhoto/{token}']['post']['security'] == []
    assert spec['paths']['/user/getInfo']['get']['security'] == [{'SessionToken': []}]


def test_docs_and_local_assets_work_without_mysql_redis_or_media(tmp_path):
    # Construct a fresh factory without any fixture's database creation or fake Redis.
    with patch('pymysql.connect', side_effect=AssertionError('Docs connected to MySQL')), \
         patch('redis.Redis.execute_command', side_effect=AssertionError('Docs connected to Redis')):
        app = create_app({'TESTING': True, 'SQLALCHEMY_DATABASE_URI': 'mysql+pymysql://invalid:invalid@127.0.0.1:1/missing',
                          'REDIS_URL': 'redis://127.0.0.1:1/0', 'UPLOAD_FOLDER': str(tmp_path / 'no-media')})
        client = app.test_client()
        for path in ('/docs', '/docs/'):
            response = client.get(path)
            assert response.status_code == 200 and response.mimetype == 'text/html'
            assert 'Location' not in response.headers
            assert '<html lang="en">' in response.text
            assets = re.findall(r'(?:src|href)="(/docs/assets/[^\"]+)"', response.text)
            assert len(assets) == 3
            assert 'https://' not in response.text and 'http://' not in response.text
            for asset in assets:
                static = client.get(asset)
                assert static.status_code == 200 and len(static.data) > 100
        document = client.get('/openapi.json')
        assert document.status_code == 200 and document.json == build_spec()
        assert not (tmp_path / 'no-media').exists()
    config = (DOCS / 'static/swagger-init.js').read_text()
    assert 'persistAuthorization: false' in config
    assert 'validatorUrl: null' in config
    assert 'localStorage' not in config and 'preauthorizeApiKey' not in config


def test_vendor_assets_are_pinned_and_unmodified():
    root = DOCS / 'static/vendor'
    source = json.loads((root / 'SOURCE.json').read_text())
    assert source['version'] == '5.32.0'
    for name, digest in source['files'].items():
        assert hashlib.sha256((root / name).read_bytes()).hexdigest() == digest
    assert 'Apache License' in (root / 'LICENSE').read_text()


def test_request_parameters_and_bodies_match_independent_contract_cases():
    from tests.contract_cases import CASES
    spec = build_spec()
    for method, path, options, _ in CASES:
        canonical = re.sub(r'/(?:token|api)-alice$', '/{token}', path)
        op = spec['paths'][canonical][method.lower()]
        query = {p['name']: p for p in op.get('parameters', []) if p['in'] == 'query'}
        for name, value in options.get('query_string', {}).items():
            assert name in query, (path, name)
            if name == 'tagIds':
                value = [int(item) for item in value.split(',')]
            validate_value(value, query[name]['schema'])
        if 'json' in options:
            validate_value(options['json'], op['requestBody']['content']['application/json']['schema'])
        if 'data' in options:
            validate_value(options['data'], op['requestBody']['content']['application/x-www-form-urlencoded']['schema'])
        if 'upload' in options:
            properties = op['requestBody']['content']['multipart/form-data']['schema']['properties']
            expected = {'avatar': {'file'}, 'one': {'file', 'photo'}, 'many': {'files', 'photosStr'}}[options['upload']]
            assert set(properties) == expected
