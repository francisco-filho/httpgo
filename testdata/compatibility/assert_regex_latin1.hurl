GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "id=(.*)" == "café"
GET http://example.invalid/health
HTTP 200
