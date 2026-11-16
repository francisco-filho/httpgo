GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "^ok$"
GET http://example.invalid/health
HTTP 200
