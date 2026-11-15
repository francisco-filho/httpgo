GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$[1]" == null
GET http://example.invalid/health
HTTP 200
