GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.n" > -1.26
jsonpath "$.n" >= -1.25
jsonpath "$.n" < -1.24
jsonpath "$.n" <= -1.25
jsonpath "$.zero" >= 0
jsonpath "$.zero" <= 0
GET http://example.invalid/health
HTTP 200
