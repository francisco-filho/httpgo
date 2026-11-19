GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(missing)" not contains "{{missing}}"
GET http://example.invalid/health
HTTP 200
