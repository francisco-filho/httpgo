GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "id=(.*)" matches "^[0-9]{7}$"
GET http://example.invalid/health
HTTP 200
