GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(id)" regex "(x)" exists
GET http://example.invalid/health
HTTP 200
