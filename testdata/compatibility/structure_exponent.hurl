GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" > 1e99999999999999999999998
GET http://example.invalid/health
HTTP 200
