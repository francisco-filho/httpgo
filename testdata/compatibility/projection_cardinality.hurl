GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.empty[*]" not exists
jsonpath "$.single[*]" == 7
jsonpath "$.many[*]" count == 2
jsonpath "$.many[*]" contains 2
jsonpath "$.nulls[*]" == null
jsonpath "$.missing[*].x" not exists
jsonpath "$.one_array[*]" count == 0
jsonpath "$.arrays[*]" count == 2
jsonpath "$.nested[*].products[*].id" contains 3
jsonpath "$.indexed[*][0]" contains 9
GET http://example.invalid/health
