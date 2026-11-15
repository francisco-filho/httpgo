GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.one" == 1.0
jsonpath "$.decimal" == 125e-2
jsonpath "$.zero" == -0.0e99
GET http://example.invalid/health
HTTP 200
