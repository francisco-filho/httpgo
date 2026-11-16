GET http://example.invalid/capture
HTTP 200
[Asserts]
body not contains "customer"
GET http://example.invalid/health
HTTP 200
