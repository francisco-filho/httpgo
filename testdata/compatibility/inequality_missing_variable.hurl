GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" != {{missing}}
GET http://example.invalid/health
