GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.token" isString
jsonpath "$.attempts" isInteger
jsonpath "$.active" isBoolean
jsonpath "$.issues" isEmpty
GET http://example.invalid/health
HTTP 200
