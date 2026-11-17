GET {{base_url}}/first
GET {{base_url}}/products
HTTP 200
[Asserts]
jsonpath "$[?(@.price > 1)].id" exists
