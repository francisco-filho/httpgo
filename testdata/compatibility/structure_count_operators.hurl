GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.empty" count == 0
jsonpath "$.empty" count >= 0
jsonpath "$.empty" count <= 0
jsonpath "$.items" count > 2
jsonpath "$.items" count < 4
jsonpath "$.items" count == 3
GET http://example.invalid/health
HTTP 200
