GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "private" "extra"
GET http://example.invalid/health
HTTP 200
