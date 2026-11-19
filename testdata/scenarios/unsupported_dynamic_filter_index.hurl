GET {{base_url}}/first
GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[{{index}}].id" exists
