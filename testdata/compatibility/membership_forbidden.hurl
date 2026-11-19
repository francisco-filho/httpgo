GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" not contains "OTHER_TENANT"
GET http://example.invalid/health
HTTP 200
