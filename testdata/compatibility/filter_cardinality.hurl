GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.none[?(@.x==1)].v" not exists
jsonpath "$.one[?(@.x==1)].v" == 7
jsonpath "$.null[?(@.x==1)].v" == null
jsonpath "$.arrays[?(@.x==1)].v" count == 2
jsonpath "$.arrays[?(@.x==1)].v[0]" == 2
jsonpath "$.single_array[?(@.x==1)].v" count == 2

GET http://example.invalid/health
HTTP 200
