// Run against the unmodified dorny/test-reporter v2.6.0 build. No application
// dependency: npm ci && npm run build are run inside that consumer's checkout.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {resolve} from 'node:path'
import {pathToFileURL} from 'node:url'

const [consumer, report] = process.argv.slice(2)
const {JavaJunitParser} = await import(pathToFileURL(resolve(consumer, 'lib/parsers/java-junit/java-junit-parser.js')))
const parser = new JavaJunitParser({trackedFiles: [], parseErrors: true})
const result = await parser.parse(report, await readFile(report, 'utf8'))
assert.equal(result.tests, 4)
assert.equal(result.passed, 1)
assert.equal(result.failed, 2) // Consumer displays both failure and error as failed.
assert.equal(result.skipped, 1)
assert.equal(result.time, 4000)
const cases = result.suites.flatMap(suite => suite.groups.flatMap(group => group.tests))
assert.deepEqual(cases.map(test => test.result), ['success', 'failed', 'failed', 'skipped'])
assert.equal(cases[0].name, 'tests/01-ação & "aspas" <ok>.hurl')
assert.equal(cases[0].time, 1250)
assert.equal(cases[1].error.details, '"tests/02-failure.hurl":4:1: assertion: esperado "ação & <x>", recebido "outro"')
assert.equal(cases[2].error.details, '"tests/03-error.hurl":1:1: falha de transporte')
console.log(`JUnit accepted by dorny/test-reporter: ${result.tests} tests, ${result.passed} passed, ${result.failed} failed/error, ${result.skipped} skipped`)
