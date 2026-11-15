GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.value" == "ok"
GET http://example.invalid/health
HTTP 200
