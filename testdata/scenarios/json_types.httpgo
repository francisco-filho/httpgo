GET {{base_url}}/session-info
HTTP 200
[Asserts]
jsonpath "$.token" isString
jsonpath "$.attempts" isInteger
jsonpath "$.active" isBoolean
jsonpath "$.issues" isEmpty
