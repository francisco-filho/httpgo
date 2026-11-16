GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "Pizza"
GET http://example.invalid/health
HTTP 200
