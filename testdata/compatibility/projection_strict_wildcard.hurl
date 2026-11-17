GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$[*]" not exists
GET http://example.invalid/health
