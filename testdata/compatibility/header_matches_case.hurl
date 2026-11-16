GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "pizza"
GET http://example.invalid/health
HTTP 200
