GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(id)" exists "x"
GET http://example.invalid/health
HTTP 200
