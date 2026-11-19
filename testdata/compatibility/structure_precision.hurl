GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" > 9007199254740992
GET http://example.invalid/health
HTTP 200
