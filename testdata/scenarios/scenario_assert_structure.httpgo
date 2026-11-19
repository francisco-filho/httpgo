GET {{base_url}}/products

HTTP 200
[Asserts]
jsonpath "$.items" count == 3
jsonpath "$.total" >= 1
jsonpath "$.items[0].id" exists
