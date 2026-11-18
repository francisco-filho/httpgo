GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(a*)b" exists
regex "(a*)b" == ""
regex "(a*)b" contains ""
GET http://example.invalid/health
HTTP 200
