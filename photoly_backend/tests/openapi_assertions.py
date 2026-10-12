"""Validate live responses independently of the documentation builders."""
import re
from functools import lru_cache
from jsonschema import Draft202012Validator
from app.docs import build_spec


@lru_cache
def specification():
    return build_spec()


def validate_value(value, schema):
    # Keep the same local JSON-pointer namespace as the OpenAPI root document.
    Draft202012Validator({**schema, 'components': specification()['components']}).validate(value)


def assert_documented_response(method, path, response):
    path = re.sub(r'/(?:token|api)-alice$', '/{token}', path)
    operation = specification()['paths'][path][method.lower()]
    definition = operation['responses'][str(response.status_code)]
    if not response.data and 'content' not in definition:
        return
    media = definition['content'][response.mimetype]
    schema = media['schema']
    if schema.get('format') == 'binary':
        assert response.data
    else:
        validate_value(response.json if response.is_json else response.text, schema)
