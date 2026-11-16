GET http://example.invalid/capture
HTTP 200
[Asserts]
body contains "customer"
body not contains "passwordHash"
body not contains "trackingToken"
GET http://example.invalid/health
HTTP 200
