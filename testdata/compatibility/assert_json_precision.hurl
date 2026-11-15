GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.items[0].id" == 9007199254740992
GET http://example.invalid/health
HTTP 200
