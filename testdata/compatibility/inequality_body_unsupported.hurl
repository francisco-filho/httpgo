GET http://example.invalid/capture
HTTP 200
[Asserts]
body != "other"
GET http://example.invalid/health
