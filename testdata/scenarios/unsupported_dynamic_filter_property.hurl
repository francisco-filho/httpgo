GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.{{property}}==1)].id" exists
