GET {{base_url}}/first
GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number {{operator}} 1)].id" exists
