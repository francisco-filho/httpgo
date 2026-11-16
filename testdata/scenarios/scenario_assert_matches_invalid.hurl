GET {{base_url}}/first
HTTP 200
GET {{base_url}}/late
HTTP *
[Asserts]
jsonpath "$.name" matches "(?=Pizza)"
