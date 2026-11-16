GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$[0]" exists
GET http://example.invalid/health
HTTP 200
