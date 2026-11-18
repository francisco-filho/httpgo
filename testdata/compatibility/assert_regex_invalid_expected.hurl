GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(missing)" matches "{{pattern}}"
GET http://example.invalid/health
HTTP 200
