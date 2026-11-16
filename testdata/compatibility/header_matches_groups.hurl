GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X-Test" matches "^(ab|cd)[0-9]{2}$"
GET http://example.invalid/health
HTTP 200
