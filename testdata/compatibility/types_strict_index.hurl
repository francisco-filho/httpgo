GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[*][0]" isEmpty
GET http://example.invalid/health
HTTP 200
