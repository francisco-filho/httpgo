GET {{base_url}}/orders
HTTP 200
[Captures]
selected: jsonpath "$.items[?(@.name=='Order {{broken')].id"
