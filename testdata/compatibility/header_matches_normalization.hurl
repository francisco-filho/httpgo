GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "^é$"
GET http://example.invalid/health
HTTP 200
