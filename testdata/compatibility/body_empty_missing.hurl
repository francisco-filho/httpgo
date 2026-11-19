GET http://example.invalid/capture
HTTP 200
[Asserts]
body contains "customer"
GET http://example.invalid/health
HTTP 200
