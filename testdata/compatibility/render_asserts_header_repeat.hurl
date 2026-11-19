GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" matches "^{{pattern}}$"
GET http://example.invalid/health
HTTP 200
