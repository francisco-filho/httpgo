GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.groups[*].items[?(@.meta.keys[0]=='pizza')].variants[?(@.code=='x')].id" count == 3
jsonpath "$.groups[*].items[?(@.meta.keys[0]=='pizza')].variants[?(@.code=='x')].id" contains 1
jsonpath "$.groups[*].items[?(@.meta.keys[0]=='pizza')].variants[?(@.code=='x')].id" contains 3

GET http://example.invalid/health
HTTP 200
