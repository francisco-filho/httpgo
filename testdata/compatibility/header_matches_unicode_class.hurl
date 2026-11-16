GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "^\\p{L}+$"
GET http://example.invalid/health
HTTP 200
