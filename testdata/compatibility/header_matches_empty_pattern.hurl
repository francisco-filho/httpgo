GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches ""
GET http://example.invalid/health
HTTP 200
