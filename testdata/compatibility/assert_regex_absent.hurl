GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(missing)" == ""
GET http://example.invalid/health
HTTP 200
