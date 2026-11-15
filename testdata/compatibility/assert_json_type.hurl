GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.price" == 4500
GET http://example.invalid/health
HTTP 200
