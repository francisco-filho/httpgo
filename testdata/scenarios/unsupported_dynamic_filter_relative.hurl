GET {{base_url}}/first
GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.meta[{{index}}]==1)].id" exists
