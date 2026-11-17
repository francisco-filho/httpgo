GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[*].v" isEmpty
GET http://example.invalid/health
HTTP 200
