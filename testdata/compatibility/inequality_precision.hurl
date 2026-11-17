GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" != 9007199254740993
GET http://example.invalid/health
