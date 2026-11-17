GET http://example.invalid/capture
HTTP 200
[Asserts]
body matches "{{pattern}}"
GET http://example.invalid/health
HTTP 200
