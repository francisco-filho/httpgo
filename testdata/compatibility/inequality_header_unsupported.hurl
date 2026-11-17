GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Probe" != "other"
GET http://example.invalid/health
