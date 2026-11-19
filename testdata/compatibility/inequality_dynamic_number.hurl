GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" != {{v}}
GET http://example.invalid/health
