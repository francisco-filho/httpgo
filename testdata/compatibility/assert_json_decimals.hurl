GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.one" == 1.0
jsonpath "$.decimal" == 1.25
jsonpath "$.zero" == -0.0
GET http://example.invalid/health
HTTP 200
