GET http://example.invalid/capture
HTTP 200
[Asserts]
header "Absent" not contains ""
GET http://example.invalid/health
HTTP 200
