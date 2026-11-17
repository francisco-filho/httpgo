GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.issues[*].code" contains "REQUIRED"
jsonpath "$.issues[*].code" not contains "OTHER_TENANT"
[Captures]
codes: jsonpath "$.issues[*].code"
POST {{base_url}}/echo
{{codes}}
HTTP 204
